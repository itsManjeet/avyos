#!/bin/bash

# Source common configuration
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/common.sh"

SYSEXT_DIR="${CACHE_PATH}/extensions/${CHANNEL}"

set -euo pipefail

ELEMENT="${1:-${ELEMENT:-}}"
if [ -z "$ELEMENT" ]; then
    echo "Error: no ELEMENT specified" >&2
    echo "Usage: $0 <element>" >&2
    exit 1
fi


# Build the specified element
"$IGNITE" build -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" "$ELEMENT"

# Create temporary directory for checking out the element
TMPDIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Checkout the element to temporary directory
"$IGNITE" checkout -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" "$ELEMENT" "$TMPDIR/sysext"

# Create extension directory if it does not exist
install -d "$SYSEXT_DIR"

# Copy, compress and hash
name=$(basename "$ELEMENT" .yml)
cp "$TMPDIR/sysext/$name.raw" "$SYSEXT_DIR/$name.raw"
xz -T0 -f -k "$SYSEXT_DIR/$name.raw"

(cd "$SYSEXT_DIR" && sha256sum *.raw.xz > SHA256SUMS)

printf 'Published sysext %s to %s\n' "$name" "$SYSEXT_DIR"
printf 'Sysext URL: https://repo.avyos.dev/releases/%s/extensions/%s.raw.xz\n' "$CHANNEL" "$name"
