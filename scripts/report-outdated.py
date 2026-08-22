#!/usr/bin/env python3
"""
Report and optionally update outdated rlxos recipes by comparing against
the latest upstream versions from Repology.

Uses the Repology /api/v1/project/ API with concurrent requests for
efficient bulk checking.

Usage:
    scripts/report-outdated.py [options]
    scripts/report-outdated.py --update
    scripts/report-outdated.py --json > outdated.json
    scripts/report-outdated.py --commit-msg

Options:
    --packages PKG,...  Only check these packages (comma-separated)
    --json              Output report as JSON
    --update            Update outdated recipe files in-place
    --commit-msg        Print a git commit message for updated recipes
    --dry-run           Show what would be updated without writing files
"""

import argparse
import json
import re
import sys
import time
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

REPOLOGY_API = "https://repology.org/api/v1/project"
USER_AGENT = "rlxos-report-outdated/1.0"


# ---------------------------------------------------------------------------
# Recipe parsing
# ---------------------------------------------------------------------------

def parse_recipe(filepath: str) -> dict | None:
    """Parse an rlxos recipe YAML file. Returns dict with id and version."""
    try:
        with open(filepath, "r") as f:
            content = f.read()
    except OSError:
        return None

    result = {"_path": filepath, "id": "", "version": ""}

    for line in content.splitlines():
        line_s = line.strip()
        if line_s.startswith("id:"):
            result["id"] = line_s[3:].strip()
        elif line_s.startswith("version:"):
            val = line_s[8:].strip()
            if "  #" in val:
                val = val[:val.index("  #")].strip()
            result["version"] = val

    return result if result["id"] and result["version"] else None


# ---------------------------------------------------------------------------
# Repology API
# ---------------------------------------------------------------------------

def _extract_newest(entries: list[dict]) -> str | None:
    """From a list of repology entries for one project, pick the newest version.
    Prefer Arch Linux's version when available."""
    arch_newest = set()
    all_newest = set()
    for e in entries:
        if e.get("status") == "newest":
            all_newest.add(e["version"])
            if e.get("repo") == "arch":
                arch_newest.add(e["version"])

    if not all_newest:
        return None

    pool = arch_newest if arch_newest else all_newest
    return sorted(pool, key=_version_sort_key, reverse=True)[0]


def fetch_newest_version(pkgname: str) -> tuple[str, str | None]:
    """Fetch newest version for a single package. Returns (pkgname, version)."""
    url = f"{REPOLOGY_API}/{urllib.request.quote(pkgname)}"
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.loads(resp.read())
    except (urllib.error.URLError, OSError, json.JSONDecodeError):
        return (pkgname, None)

    if not isinstance(data, list):
        return (pkgname, None)

    return (pkgname, _extract_newest(data))


def fetch_all_versions(pkg_ids: set[str], quiet: bool = False,
                       max_workers: int = 1) -> dict[str, str]:
    """Fetch newest versions for all given package IDs concurrently.
    Returns a dict of {pkg_id: newest_version}."""
    result = {}
    total = len(pkg_ids)
    done = 0

    with ThreadPoolExecutor(max_workers=max_workers) as pool:
        futures = {pool.submit(fetch_newest_version, pid): pid
                   for pid in pkg_ids}

        for future in as_completed(futures):
            done += 1
            pkgname, version = future.result()
            if version:
                result[pkgname] = version

            if not quiet:
                print(f"\r[{done}/{total}] fetched {pkgname}",
                      end="", flush=True, file=sys.stderr)

    if not quiet:
        print("\r" + " " * 60 + "\r", end="", file=sys.stderr)

    return result


# ---------------------------------------------------------------------------
# Version comparison
# ---------------------------------------------------------------------------

def _version_parts(v: str) -> list:
    """Split version string into comparable parts."""
    parts = []
    for seg in re.split(r'[.\-_+]', v):
        if seg.isdigit():
            parts.append(int(seg))
        else:
            parts.append(seg)
    return parts


def _version_sort_key(v: str) -> list:
    """Sort key that puts numeric segments first for proper ordering."""
    parts = []
    for seg in re.split(r'[.\-_+]', v):
        if seg.isdigit():
            parts.append((0, int(seg)))
        else:
            parts.append((1, seg))
    return parts


def version_newer(upstream: str, current: str) -> bool:
    """Return True if upstream is strictly newer than current."""
    if upstream == current:
        return False
    up = _version_parts(upstream)
    cur = _version_parts(current)
    max_len = max(len(up), len(cur))
    for i in range(max_len):
        a = up[i] if i < len(up) else 0
        b = cur[i] if i < len(cur) else 0
        if type(a) != type(b):
            if isinstance(a, int):
                return True
            return False
        if a > b:
            return True
        if a < b:
            return False
    return False


# ---------------------------------------------------------------------------
# Recipe updater
# ---------------------------------------------------------------------------

def update_recipe_version(filepath: str, old_version: str,
                          new_version: str) -> bool:
    """Update the version in a recipe file. Returns True on success."""
    try:
        with open(filepath, "r") as f:
            content = f.read()
    except OSError:
        return False

    new_content = re.sub(
        r'^(version:\s*)' + re.escape(old_version) + r'(\s*(?:#.*)?)$',
        rf'\g<1>{new_version}\2',
        content, count=1, flags=re.MULTILINE)

    # Also update source URLs that embed the old version literally
    new_content = new_content.replace(old_version, new_version)

    if new_content == content:
        return False

    with open(filepath, "w") as f:
        f.write(new_content)
    return True


# ---------------------------------------------------------------------------
# Report generation
# ---------------------------------------------------------------------------

def generate_commit_msg(outdated: list[dict]) -> str:
    """Generate a git commit message for updated recipes."""
    if not outdated:
        return ""

    if len(outdated) == 1:
        r = outdated[0]
        return (f"update {r['id']} to {r['upstream_version']}\n\n"
                f"Updated from {r['current_version']} to "
                f"{r['upstream_version']}.")

    lines = [f"update {len(outdated)} outdated packages", ""]
    lines.append("Updated packages:")
    for r in sorted(outdated, key=lambda x: x["id"]):
        lines.append(
            f"  - {r['id']}: {r['current_version']} -> {r['upstream_version']}")
    lines.append("")
    return "\n".join(lines)


def generate_json_report(outdated: list[dict]) -> str:
    """Generate a JSON report of outdated packages."""
    report = {
        "total_outdated": len(outdated),
        "packages": sorted(outdated, key=lambda x: x["id"]),
    }
    return json.dumps(report, indent=2)


def print_table(outdated: list[dict]) -> None:
    """Print a human-readable table of outdated packages."""
    if not outdated:
        print("All packages are up to date.")
        return

    max_id = max(len(r["id"]) for r in outdated)
    max_cur = max(len(r["current_version"]) for r in outdated)
    max_new = max(len(r["upstream_version"]) for r in outdated)

    header = (f"{'Package':<{max_id}}  "
              f"{'Current':<{max_cur}}  "
              f"{'Upstream':<{max_new}}")
    print(header)
    print("-" * len(header))

    for r in sorted(outdated, key=lambda x: x["id"]):
        print(f"{r['id']:<{max_id}}  "
              f"{r['current_version']:<{max_cur}}  "
              f"{r['upstream_version']:<{max_new}}")

    print(f"\n{len(outdated)} outdated package(s) found.")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Report and update outdated rlxos recipes")
    parser.add_argument("--packages",
                        help="Comma-separated list of packages to check")
    parser.add_argument("--json", action="store_true",
                        help="Output report as JSON")
    parser.add_argument("--update", action="store_true",
                        help="Update recipe files to latest version")
    parser.add_argument("--commit-msg", action="store_true",
                        help="Print a git commit message for updates")
    parser.add_argument("--dry-run", action="store_true",
                        help="Show updates without writing files")
    args = parser.parse_args()

    project_root = Path(__file__).resolve().parent.parent
    recipes_dir = project_root / "elements" / "components"

    if not recipes_dir.is_dir():
        print(f"Error: recipes directory not found: {recipes_dir}",
              file=sys.stderr)
        sys.exit(1)

    # Collect recipe files to check
    if args.packages:
        pkg_names = [p.strip() for p in args.packages.split(",")]
        recipe_files = []
        for name in pkg_names:
            path = recipes_dir / f"{name}.yml"
            if path.exists():
                recipe_files.append(path)
            else:
                print(f"WARN: recipe not found: {path}", file=sys.stderr)
    else:
        recipe_files = sorted(recipes_dir.glob("*.yml"))

    # Parse all recipes
    recipes = {}  # pkg_id -> {version, path}
    skipped = 0
    for rpath in recipe_files:
        recipe = parse_recipe(str(rpath))
        if not recipe or recipe["version"] == "FIXME":
            skipped += 1
            continue
        recipes[recipe["id"]] = {
            "version": recipe["version"],
            "path": str(rpath),
        }

    quiet = args.json
    upstream_versions = fetch_all_versions(set(recipes.keys()), quiet=quiet)

    # Compare versions
    outdated = []
    for pkg_id, info in sorted(recipes.items()):
        upstream = upstream_versions.get(pkg_id)
        if not upstream:
            continue
        if version_newer(upstream, info["version"]):
            outdated.append({
                "id": pkg_id,
                "current_version": info["version"],
                "upstream_version": upstream,
                "recipe_path": info["path"],
            })

    checked = len(recipes)

    # Apply updates if requested
    if args.update or args.commit_msg:
        updated = []
        for entry in outdated:
            if args.dry_run:
                print(f"DRY-RUN: would update {entry['id']} "
                      f"{entry['current_version']} -> "
                      f"{entry['upstream_version']}")
                updated.append(entry)
            else:
                ok = update_recipe_version(
                    entry["recipe_path"],
                    entry["current_version"],
                    entry["upstream_version"])
                if ok:
                    updated.append(entry)
                    if not args.json:
                        print(f"UPDATED {entry['id']}: "
                              f"{entry['current_version']} -> "
                              f"{entry['upstream_version']}")
                else:
                    print(f"FAIL    {entry['id']}: could not update",
                          file=sys.stderr)

        if args.commit_msg:
            msg = generate_commit_msg(updated)
            if msg:
                print(msg)
            return

    # Output report
    if args.json:
        print(generate_json_report(outdated))
    elif not args.commit_msg:
        print_table(outdated)

    print(f"\nChecked {checked}, skipped {skipped}, "
          f"outdated {len(outdated)}", file=sys.stderr)


if __name__ == "__main__":
    main()
