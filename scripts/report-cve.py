#!/usr/bin/env python3
"""
Generate a CVE report for rlxos external recipes.

The script scans `external/*/recipe.yml`, extracts package IDs and
versions, and queries OSV for known vulnerabilities affecting those versions.

Usage:
    scripts/report-cve.py
    scripts/report-cve.py --packages openssl,nano
    scripts/report-cve.py --json

Notes:
    Matching is best-effort and depends on upstream package naming used by OSV.
"""

import argparse
import contextlib
import io
import json
import sys
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

from common import apply

OSV_QUERY_API = "https://api.osv.dev/v1/query"
USER_AGENT = "rlxos-report-cve/1.0"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate a CVE report")
    parser.add_argument(
        "--packages",
        help="Only check these packages (comma-separated)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print report as JSON",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=8,
        help="Number of concurrent API requests (default: 8)",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=20,
        help="HTTP timeout in seconds (default: 20)",
    )
    return parser.parse_args()


def collect_packages(selected: set[str] | None) -> list[dict]:
    packages = []

    def callback(filename: str, data: dict):
        if "id" not in data or "version" not in data:
            return

        name = str(data["id"]).strip()
        version = str(data["version"]).strip()
        if not name or not version:
            return
        if selected and name not in selected:
            return

        packages.append({
            "id": name,
            "version": version,
            "path": filename,
        })

    with contextlib.redirect_stdout(io.StringIO()):
        apply(callback)
    return packages


def _extract_aliases(vuln: dict) -> list[str]:
    aliases = []
    if vuln.get("id"):
        aliases.append(vuln["id"])

    for alias in vuln.get("aliases", []):
        if alias not in aliases:
            aliases.append(alias)

    return aliases


def _extract_cves(vuln: dict) -> list[str]:
    return [alias for alias in _extract_aliases(vuln) if alias.startswith("CVE-")]


def _extract_severity(vuln: dict) -> str | None:
    severities = vuln.get("severity") or []
    if severities:
        score = severities[0].get("score")
        if score:
            return score

    database_specific = vuln.get("database_specific") or {}
    severity = database_specific.get("severity")
    if isinstance(severity, str) and severity:
        return severity

    return None


def _extract_summary(vuln: dict) -> str:
    summary = vuln.get("summary")
    if isinstance(summary, str) and summary.strip():
        return summary.strip()

    details = vuln.get("details")
    if isinstance(details, str) and details.strip():
        first_line = details.strip().splitlines()[0].strip()
        if first_line:
            return first_line

    return "No summary available"


def check_cve(name: str, version: str, timeout: int) -> list[dict] | None:
    payload = json.dumps({
        "version": version,
        "package": {
            "name": name,
        },
    }).encode("utf-8")

    request = urllib.request.Request(
        OSV_QUERY_API,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "User-Agent": USER_AGENT,
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            data = json.loads(response.read().decode("utf-8"))
    except (urllib.error.URLError, TimeoutError, OSError, json.JSONDecodeError):
        return None

    vulns = data.get("vulns")
    if not isinstance(vulns, list) or not vulns:
        return []

    normalized = []
    seen = set()
    for vuln in vulns:
        aliases = _extract_aliases(vuln)
        key = tuple(sorted(aliases))
        if key in seen:
            continue
        seen.add(key)

        normalized.append({
            "id": vuln.get("id", "UNKNOWN"),
            "cves": _extract_cves(vuln),
            "aliases": aliases,
            "summary": _extract_summary(vuln),
            "severity": _extract_severity(vuln),
            "modified": vuln.get("modified"),
            "published": vuln.get("published"),
            "references": [
                ref.get("url") for ref in vuln.get("references", [])
                if isinstance(ref, dict) and ref.get("url")
            ],
        })

    return normalized


def scan_packages(packages: list[dict], timeout: int, workers: int) -> tuple[list[dict], list[dict]]:
    vulnerable = []
    errors = []
    total = len(packages)
    done = 0

    with ThreadPoolExecutor(max_workers=max(1, workers)) as pool:
        futures = {
            pool.submit(check_cve, pkg["id"], pkg["version"], timeout): pkg
            for pkg in packages
        }

        for future in as_completed(futures):
            done += 1
            pkg = futures[future]
            print(
                f"\r[{done}/{total}] checked {pkg['id']}",
                end="",
                flush=True,
                file=sys.stderr,
            )

            try:
                vulnerabilities = future.result()
            except Exception as exc:
                vulnerabilities = None
                errors.append({
                    "id": pkg["id"],
                    "version": pkg["version"],
                    "path": pkg["path"],
                    "error": str(exc),
                })

            if vulnerabilities is None:
                errors.append({
                    "id": pkg["id"],
                    "version": pkg["version"],
                    "path": pkg["path"],
                    "error": "request failed",
                })
                continue

            if vulnerabilities:
                vulnerable.append({
                    "id": pkg["id"],
                    "version": pkg["version"],
                    "path": pkg["path"],
                    "vulnerabilities": vulnerabilities,
                })

    if total:
        print("\r" + " " * 80 + "\r", end="", file=sys.stderr)

    vulnerable.sort(key=lambda item: item["id"])
    errors.sort(key=lambda item: item["id"])
    return vulnerable, errors


def generate_report(vulnerable: list[dict], errors: list[dict], scanned: int) -> dict:
    advisory_count = sum(len(pkg["vulnerabilities"]) for pkg in vulnerable)
    cve_count = 0
    for pkg in vulnerable:
        for vuln in pkg["vulnerabilities"]:
            cve_count += len(vuln["cves"]) if vuln["cves"] else 1

    return {
        "scanned_packages": scanned,
        "vulnerable_packages": len(vulnerable),
        "advisories": advisory_count,
        "cves": cve_count,
        "packages": vulnerable,
        "errors": errors,
    }


def print_human_report(report: dict) -> None:
    print(f"Scanned packages: {report['scanned_packages']}")
    print(f"Vulnerable packages: {report['vulnerable_packages']}")
    print(f"Advisories: {report['advisories']}")
    print(f"CVEs: {report['cves']}")
    print(f"Errors: {len(report['errors'])}")

    if not report["packages"]:
        return

    print("")
    for pkg in report["packages"]:
        print(f"{pkg['id']} {pkg['version']} ({pkg['path']})")
        for vuln in pkg["vulnerabilities"]:
            title = ", ".join(vuln["cves"]) if vuln["cves"] else vuln["id"]
            line = f"  - {title}"
            if vuln["severity"]:
                line += f" [{vuln['severity']}]"
            print(line)
            print(f"    {vuln['summary']}")
            if vuln["references"]:
                print(f"    {vuln['references'][0]}")
        print("")

    if report["errors"]:
        print("Lookup errors:")
        for error in report["errors"]:
            print(f"  - {error['id']} {error['version']}: {error['error']}")


def main() -> int:
    args = parse_args()
    selected = None
    if args.packages:
        selected = {pkg.strip() for pkg in args.packages.split(",") if pkg.strip()}

    print("checking CVE packages", file=sys.stderr)
    packages = collect_packages(selected)
    report = generate_report(*scan_packages(packages, args.timeout, args.workers), len(packages))

    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print_human_report(report)

    return 0 if not report["errors"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
