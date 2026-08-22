#!/bin/bash

# Source common configuration
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/common.sh"

RELEASE_DIR="${CACHE_PATH}/releases/${CHANNEL}"

set -euo pipefail

# Create temporary directory for checking out the installer image
TMPDIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Checkout installer image to temporary directory
"$IGNITE" checkout -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" installer "$TMPDIR"

# Create release directory if it does not exist
install -d "$RELEASE_DIR"

ISO_NAME="rlxos-${VERSION}-${CHANNEL}-installer.iso"

# Copy installer ISO to release directory
cp "$TMPDIR/rlxos-${CHANNEL}-installer.iso" "$RELEASE_DIR/$ISO_NAME"

# Generate zsync file
(cd "$RELEASE_DIR" && zsyncmake -b 2048 -C -u "http://repo.rlxos.org/releases/${CHANNEL}/${ISO_NAME}" "$ISO_NAME")

printf 'Published installer ISO to %s\n' "$RELEASE_DIR"
printf 'ISO URL: http://repo.rlxos.org/releases/%s/%s\n' "$CHANNEL" "$ISO_NAME"
