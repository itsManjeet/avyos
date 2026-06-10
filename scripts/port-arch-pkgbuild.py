#!/usr/bin/env python3
"""Port Arch Linux PKGBUILDs (in arch-repo/) to avyos recipes (elements/components/).

Local sources referenced by the PKGBUILD are copied:
  - *.patch / *.diff        -> patches/<pkgname>/
  - everything else (local) -> files/<pkgname>/

Existing elements/components/<pkgname>.yml files are NOT overwritten
unless --overwrite is passed.

Usage:
  scripts/port-arch-pkgbuild.py PKGNAME [PKGNAME...]
  scripts/port-arch-pkgbuild.py --all
  scripts/port-arch-pkgbuild.py --from-list FILE
"""

import argparse
import os
import re
import shlex
import shutil
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
ARCH_REPO = REPO_ROOT / "arch-repo"
COMPONENTS_DIR = REPO_ROOT / "elements" / "components"
PATCHES_DIR = REPO_ROOT / "patches"
FILES_DIR = REPO_ROOT / "files"

# Deps that match these substrings are almost always .so soname deps or Arch
# packaging artifacts - skip them entirely from the ported recipe.
SKIP_DEP_PATTERNS = (
    re.compile(r"\.so(?:\.|$)"),     # libfoo.so, libfoo.so.1
    re.compile(r"^lib.+\.so$"),
)

# Optional coarse name translation from Arch conventions to avyos components.
# Intentionally minimal - mapping is a hint, not authoritative. The porter
# emits components/<name>.yml for each dep; the user fixes missing ones.
DEP_TRANSLATE = {
    "glib2": "glib",
    "glib2-devel": "glib",
    "gtk3-devel": "gtk3",
    "gtk4-devel": "gtk4",
    "libgl": "mesa",
    "mesa-libgl": "mesa",
    "wayland-protocols": "wayland-protocols",
    "python": "python",
    "python3": "python",
}


def die(msg):
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


def warn(msg):
    print(f"warn: {msg}", file=sys.stderr)


def info(msg):
    print(msg)


# ---------------------------------------------------------------------------
# PKGBUILD parsing
# ---------------------------------------------------------------------------

def source_pkgbuild(pkgbuild_path: Path) -> dict:
    """Extract the declared variables from a PKGBUILD by sourcing it in bash.

    Functions (prepare/build/check/package) are defined but not invoked, so
    sourcing is safe. We then use `declare -p` for each variable of interest
    and parse the bash output.
    """
    wanted = [
        "pkgname", "pkgbase", "pkgver", "pkgrel", "pkgdesc",
        "url", "arch", "license",
        "depends", "makedepends", "checkdepends", "optdepends",
        "source", "install", "backup", "provides", "conflicts", "replaces",
    ]

    declares = "\n".join(f"declare -p {v} 2>/dev/null || true" for v in wanted)
    script = f"""
set +e
# CARCH/CHOST are normally set by makepkg
export CARCH=x86_64 CHOST=x86_64-pc-linux-gnu
source {shlex.quote(str(pkgbuild_path))} >/dev/null 2>&1
{declares}
"""
    try:
        out = subprocess.check_output(["bash", "-c", script], text=True,
                                      stderr=subprocess.DEVNULL)
    except subprocess.CalledProcessError as e:
        die(f"failed to source {pkgbuild_path}: {e}")

    result = {}
    for line in out.splitlines():
        parsed = parse_declare(line)
        if parsed is None:
            continue
        name, value = parsed
        result[name] = value
    return result


_DECLARE_RE = re.compile(r"^declare -([a-zA-Z-]+) ([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?$")


def parse_declare(line: str):
    """Parse a single 'declare -p' line.

    Returns (name, value) where value is:
      - str for scalars
      - list[str] for indexed arrays
    Returns None for lines that aren't declares (e.g. blank, 'not found').
    """
    m = _DECLARE_RE.match(line)
    if not m:
        return None
    flags, name, rhs = m.group(1), m.group(2), m.group(3)
    if rhs is None:
        return name, "" if "a" not in flags and "A" not in flags else []
    if "a" in flags:
        return name, parse_bash_array(rhs)
    if "A" in flags:
        # associative - we don't care
        return None
    # scalar - rhs is a quoted bash value
    return name, bash_unquote(rhs)


def bash_unquote(s: str) -> str:
    """Best-effort unquote a bash-quoted string produced by 'declare -p'.

    bash uses ANSI-C quoting ($'...') or plain double quotes.
    """
    s = s.strip()
    if not s:
        return ""
    if s.startswith("$'") and s.endswith("'"):
        body = s[2:-1]
        return decode_ansi_c(body)
    if s.startswith('"') and s.endswith('"'):
        body = s[1:-1]
        # Unescape the common backslash-escapes bash uses in "..."
        return re.sub(r'\\(["\\$`])', r'\1', body)
    if s.startswith("'") and s.endswith("'"):
        return s[1:-1]
    return s


def decode_ansi_c(s: str) -> str:
    out = []
    i = 0
    while i < len(s):
        c = s[i]
        if c == "\\" and i + 1 < len(s):
            n = s[i + 1]
            mapping = {"n": "\n", "t": "\t", "r": "\r", "\\": "\\",
                       "'": "'", '"': '"', "a": "\a", "b": "\b", "f": "\f",
                       "v": "\v", "0": "\0"}
            if n in mapping:
                out.append(mapping[n])
                i += 2
                continue
        out.append(c)
        i += 1
    return "".join(out)


def parse_bash_array(s: str) -> list:
    """Parse '([0]="a" [1]="b")' or '("a" "b" "c")' into a Python list."""
    s = s.strip()
    if s.startswith("(") and s.endswith(")"):
        s = s[1:-1]
    # shlex handles the quoted-token case well enough
    lexer = shlex.shlex(s, posix=True)
    lexer.whitespace = " \t\n"
    lexer.whitespace_split = True
    lexer.commenters = ""
    tokens = []
    for tok in lexer:
        # Strip "[N]=" prefix if present
        m = re.match(r"^\[\d+\]=(.*)$", tok, re.DOTALL)
        if m:
            tok = m.group(1)
        tokens.append(tok)
    return tokens


def extract_function(text: str, name: str) -> str | None:
    """Extract the body of a bash function `name() { ... }` from PKGBUILD text.

    Returns the inner body without the enclosing braces, or None if not found.
    """
    # Match "name() {" or "name () {" at start of line. Body continues until
    # the matching closing brace at the outermost nesting level.
    pat = re.compile(rf"^{re.escape(name)}\s*\(\s*\)\s*\{{", re.MULTILINE)
    m = pat.search(text)
    if not m:
        return None
    start = m.end()
    depth = 1
    i = start
    while i < len(text) and depth > 0:
        c = text[i]
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                break
        elif c == "'":
            # skip single-quoted (no escapes in bash '...')
            j = text.find("'", i + 1)
            if j == -1:
                break
            i = j
        elif c == '"':
            # skip double-quoted body (handle \")
            j = i + 1
            while j < len(text):
                if text[j] == "\\":
                    j += 2
                    continue
                if text[j] == '"':
                    break
                j += 1
            i = j
        elif c == "#" and (i == start or text[i - 1] in " \t\n"):
            # skip to end of line (only a comment at a word boundary)
            j = text.find("\n", i)
            if j == -1:
                break
            i = j
        i += 1
    if depth != 0:
        return None
    body = text[start:i]
    return dedent(body).strip("\n")


def dedent(s: str) -> str:
    """Remove the common leading-whitespace indent from a block of text."""
    lines = s.splitlines()
    # Drop purely empty leading/trailing lines for indent measurement
    non_empty = [ln for ln in lines if ln.strip()]
    if not non_empty:
        return s
    indent = min(len(ln) - len(ln.lstrip(" \t")) for ln in non_empty)
    if indent == 0:
        return s
    return "\n".join(ln[indent:] if len(ln) >= indent else ln for ln in lines)


# ---------------------------------------------------------------------------
# Transformations applied to function bodies when emitting script blocks
# ---------------------------------------------------------------------------

def transform_script(body: str, pkgname: str, pkgver: str) -> str:
    """Convert a PKGBUILD function body to an avyos recipe script snippet."""
    s = body

    # pkgdir -> install-root
    s = re.sub(r'\$\{pkgdir\}', '%{install-root}', s)
    s = re.sub(r'\$pkgdir\b', '%{install-root}', s)

    # srcdir -> extracted source root. We run inside the source dir already,
    # so ${srcdir}/X becomes X; bare ${srcdir} becomes "."
    s = re.sub(r'"\$\{srcdir\}/', '"', s)
    s = re.sub(r'"\$srcdir/', '"', s)
    s = re.sub(r'\$\{srcdir\}/', '', s)
    s = re.sub(r'\$srcdir/', '', s)
    s = re.sub(r'\$\{srcdir\}', '.', s)
    s = re.sub(r'\$srcdir\b', '.', s)

    # pkgname / pkgver - use literals / placeholders so the recipe works
    # without those bash vars defined.
    s = re.sub(r'\$\{pkgname\}', pkgname, s)
    s = re.sub(r'\$pkgname\b', pkgname, s)
    s = re.sub(r'\$\{pkgver\}', '%{version}', s)
    s = re.sub(r'\$pkgver\b', '%{version}', s)

    # Strip the leading `cd "<pkgname>-<pkgver>"` or `cd <pkgname>` that every
    # PKGBUILD function starts with - avyos already puts us in the extracted
    # source directory.
    src_dir_patterns = [
        rf'^\s*cd\s+"{re.escape(pkgname)}-%\{{version\}}"\s*\n',
        rf"^\s*cd\s+'{re.escape(pkgname)}-%\{{version\}}'\s*\n",
        rf'^\s*cd\s+{re.escape(pkgname)}-%\{{version\}}\s*\n',
        rf'^\s*cd\s+"{re.escape(pkgname)}"\s*\n',
        rf"^\s*cd\s+'{re.escape(pkgname)}'\s*\n",
        rf'^\s*cd\s+{re.escape(pkgname)}\s*\n',
        r'^\s*cd\s+"\.?"\s*\n',
        r"^\s*cd\s+'\.?'\s*\n",
        r'^\s*cd\s+\.\s*\n',
    ]
    for p in src_dir_patterns:
        s = re.sub(p, '', s, flags=re.MULTILINE)

    return s.rstrip() + "\n"


# ---------------------------------------------------------------------------
# Source / dep handling
# ---------------------------------------------------------------------------

def clean_dep(raw: str) -> str | None:
    """Normalize a dep string. Returns None if the dep should be dropped."""
    if not raw:
        return None
    # Drop anything that looks like a .so soname
    for p in SKIP_DEP_PATTERNS:
        if p.search(raw):
            return None
    # optdepends use "name: description" - keep just the name
    raw = raw.split(":", 1)[0]
    # Strip version constraints: name>=1.0, name=1.0, name<2.0
    raw = re.split(r'[<>=!]', raw, maxsplit=1)[0].strip()
    if not raw:
        return None
    return DEP_TRANSLATE.get(raw, raw)


def is_url_source(s: str) -> bool:
    url = s.split("::", 1)[-1]
    return url.startswith(("http://", "https://", "ftp://", "git+",
                           "git://", "svn+", "hg+", "bzr+"))


def source_basename(s: str) -> str:
    """Bash-expansion-free basename of a source entry."""
    if "::" in s:
        return s.split("::", 1)[0]
    # Drop anchors like "#tag=..." and "?..."
    cleaned = re.split(r'[#?]', s, maxsplit=1)[0]
    return os.path.basename(cleaned)


def normalize_url(url: str, pkgname: str, pkgver: str) -> str:
    """Substitute pkgname/pkgver occurrences in a source URL with avyos
    placeholders so bumping `version:` is enough to refresh the URL."""
    # Prefer ${var} forms first since they are unambiguous
    url = url.replace("${pkgname}", "%{id}")
    url = url.replace("${pkgver}", "%{version}")
    url = re.sub(r'\$pkgname\b', '%{id}', url)
    url = re.sub(r'\$pkgver\b', '%{version}', url)
    # Also replace the literal resolved values - PKGBUILD arrays come from
    # `declare -p` already expanded, so ${pkgver} is gone before we see it.
    if pkgver:
        url = url.replace(pkgver, "%{version}")
    if pkgname:
        url = re.sub(rf'(?<![A-Za-z0-9_]){re.escape(pkgname)}(?![A-Za-z0-9_])',
                     "%{id}", url)
    return url


def copy_local_source(pkgname: str, src_dir: Path, filename: str) -> str:
    """Copy a local source file from the arch-repo package dir into either
    patches/<pkgname>/ or files/<pkgname>/ and return the in-recipe path."""
    src = src_dir / filename
    if not src.exists():
        warn(f"  local source not found: {src}")
        # Fall back to a likely path so the user can fix it
        kind = "patches" if filename.endswith((".patch", ".diff")) else "files"
        return f"{kind}/{pkgname}/{filename}"

    if filename.endswith((".patch", ".diff")):
        dst_dir = PATCHES_DIR / pkgname
        rel = f"patches/{pkgname}/{filename}"
    else:
        dst_dir = FILES_DIR / pkgname
        rel = f"files/{pkgname}/{filename}"

    dst_dir.mkdir(parents=True, exist_ok=True)
    dst = dst_dir / filename
    if not dst.exists():
        shutil.copy2(src, dst)
    return rel


def process_sources(sources: list, pkgname: str, pkgver: str,
                    pkg_dir: Path) -> list:
    """Convert PKGBUILD source entries into avyos recipe source entries.

    Local files are copied into patches/ or files/ and returned as relative
    project paths.
    """
    result = []
    for s in sources:
        if not s:
            continue
        if is_url_source(s):
            # Handle bash brace expansion like `url.tar.gz{,.sig}` by keeping
            # only the primary artifact (first expansion) and dropping sig/asc.
            url = s.split("::", 1)[-1]
            # {,.sig} / {,.asc} -> drop the ,.sig variant
            url = re.sub(r'\{,\.(?:sig|asc)\}', '', url)
            url = re.sub(r'\{\.(?:sig|asc),\}', '', url)
            url = normalize_url(url, pkgname, pkgver)
            result.append(url)
        else:
            filename = source_basename(s)
            rel = copy_local_source(pkgname, pkg_dir, filename)
            result.append(rel)
    return result


# ---------------------------------------------------------------------------
# Recipe emission
# ---------------------------------------------------------------------------

def yaml_escape_scalar(s: str) -> str:
    """Produce a YAML-safe scalar rendering of `s` for simple fields."""
    if s is None:
        return '""'
    if s == "":
        return '""'
    if "\n" in s:
        # caller should have used a block scalar for multi-line
        return '"' + s.replace('\\', '\\\\').replace('"', '\\"') + '"'
    needs_quote = any(c in s for c in ":#&*?|<>=!%@`") or s.startswith(("-", " ")) or s.endswith(" ")
    if needs_quote:
        return '"' + s.replace('\\', '\\\\').replace('"', '\\"') + '"'
    return s


def indent_block(s: str, indent: int = 2) -> str:
    pad = " " * indent
    return "\n".join(pad + ln if ln else ln for ln in s.splitlines())


def build_recipe(pkgbuild_path: Path) -> str:
    text = pkgbuild_path.read_text()
    vars = source_pkgbuild(pkgbuild_path)

    pkgname_val = vars.get("pkgname")
    if isinstance(pkgname_val, list):
        # Split package - use pkgbase if present, else first element
        pkgbase = vars.get("pkgbase") or (pkgname_val[0] if pkgname_val else None)
        pkgname = pkgbase
    else:
        pkgname = pkgname_val or vars.get("pkgbase")

    if not pkgname:
        die(f"could not determine pkgname from {pkgbuild_path}")

    pkgver = vars.get("pkgver") or ""
    pkgdesc = vars.get("pkgdesc") or ""

    def as_list(v):
        if v is None:
            return []
        if isinstance(v, list):
            return v
        if isinstance(v, str):
            return [v] if v else []
        return []

    raw_depends = as_list(vars.get("depends"))
    raw_makedepends = as_list(vars.get("makedepends"))
    raw_sources = as_list(vars.get("source"))
    raw_backup = as_list(vars.get("backup"))

    depends = [d for d in (clean_dep(x) for x in raw_depends) if d]
    build_depends = [d for d in (clean_dep(x) for x in raw_makedepends) if d]

    sources = process_sources(raw_sources, pkgname, pkgver,
                              pkgbuild_path.parent)

    # Function bodies
    prepare_body = extract_function(text, "prepare")
    build_body = extract_function(text, "build")
    package_body = extract_function(text, "package")
    # split-package PKGBUILDs use package_<name>
    if package_body is None:
        m = re.search(r"^(package_[A-Za-z0-9_-]+)\s*\(\s*\)", text, re.MULTILINE)
        if m:
            package_body = extract_function(text, m.group(1))

    pre_script = transform_script(prepare_body, pkgname, pkgver) if prepare_body else ""
    combined_body = ""
    if build_body:
        combined_body += transform_script(build_body, pkgname, pkgver)
    if package_body:
        if combined_body and not combined_body.endswith("\n\n"):
            combined_body += "\n"
        combined_body += transform_script(package_body, pkgname, pkgver)

    # Compose YAML
    out = []
    out.append(f"id: {yaml_escape_scalar(pkgname)}")
    out.append(f"version: {yaml_escape_scalar(pkgver)}")
    if pkgdesc:
        out.append(f"about: {yaml_escape_scalar(pkgdesc)}")
    out.append("")

    if pre_script.strip():
        out.append("pre-script: |")
        out.append(indent_block(pre_script.rstrip("\n"), 2))
        out.append("")

    if combined_body.strip():
        out.append("script: |")
        out.append(indent_block(combined_body.rstrip("\n"), 2))
        out.append("")

    if raw_backup:
        out.append("backup:")
        for b in raw_backup:
            out.append(f"  - {yaml_escape_scalar(b)}")
        out.append("")

    if depends:
        out.append("depends:")
        for d in depends:
            out.append(f"  - components/{d}.yml")
        out.append("")

    if build_depends:
        out.append("build-depends:")
        for d in build_depends:
            out.append(f"  - components/{d}.yml")
        out.append("")

    if sources:
        out.append("sources:")
        for s in sources:
            out.append(f"  - {yaml_escape_scalar(s)}")
        out.append("")

    # collapse runs of blank lines
    text_out = "\n".join(out)
    text_out = re.sub(r"\n{3,}", "\n\n", text_out).rstrip() + "\n"
    return text_out


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------

def port_one(pkgname: str, overwrite: bool) -> bool:
    pkg_dir = ARCH_REPO / pkgname
    pkgbuild = pkg_dir / "PKGBUILD"
    if not pkgbuild.is_file():
        warn(f"{pkgname}: no PKGBUILD at {pkgbuild}")
        return False

    out_path = COMPONENTS_DIR / f"{pkgname}.yml"
    if out_path.exists() and not overwrite:
        info(f"{pkgname}: skip (recipe exists)")
        return False

    try:
        yaml_text = build_recipe(pkgbuild)
    except Exception as e:
        warn(f"{pkgname}: failed to port: {e}")
        return False

    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(yaml_text)
    info(f"{pkgname}: wrote {out_path.relative_to(REPO_ROOT)}")
    return True


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("pkgs", nargs="*", help="package names under arch-repo/")
    ap.add_argument("--all", action="store_true",
                    help="port every package under arch-repo/")
    ap.add_argument("--from-list", metavar="FILE",
                    help="read package names (one per line) from FILE")
    ap.add_argument("--overwrite", action="store_true",
                    help="overwrite existing elements/components/*.yml")
    args = ap.parse_args()

    if not ARCH_REPO.is_dir():
        die(f"{ARCH_REPO} does not exist")

    pkgs = list(args.pkgs)
    if args.from_list:
        with open(args.from_list) as f:
            pkgs.extend(ln.strip() for ln in f if ln.strip() and not ln.startswith("#"))
    if args.all:
        pkgs.extend(sorted(p.name for p in ARCH_REPO.iterdir()
                           if p.is_dir() and (p / "PKGBUILD").is_file()))

    if not pkgs:
        ap.error("no packages given; pass names, --all, or --from-list")

    # de-dup while preserving order
    seen = set()
    uniq = []
    for p in pkgs:
        if p not in seen:
            uniq.append(p)
            seen.add(p)

    written = 0
    for p in uniq:
        if port_one(p, args.overwrite):
            written += 1
    info(f"done: {written}/{len(uniq)} recipes written")


if __name__ == "__main__":
    main()
