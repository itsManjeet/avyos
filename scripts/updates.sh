#!/bin/bash

# Source common configuration
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/common.sh"

UPDATES_DIR="${CACHE_PATH}/updates/${CHANNEL}"
RELEASE_DIR="${CACHE_PATH}/releases/${CHANNEL}"

set -euo pipefail

# Build the system image
"$IGNITE" build -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" system/image.yml

# Create temporary directory for checking out elements
TMPDIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Checkout system/usr.yml and system/image.yml to temporary directory
"$IGNITE" checkout -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" system/usr.yml "$TMPDIR/system"
"$IGNITE" checkout -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" system/image.yml "$TMPDIR/image"

# Create directories if they do not exist
install -d "$UPDATES_DIR" "$RELEASE_DIR"

# Resolve usr.squashfs and usr.verity symlinks
usr_image=$(readlink "$TMPDIR/system/usr.squashfs")
usr_verity=$(readlink "$TMPDIR/system/usr.verity")

test -n "$usr_image"
test -n "$usr_verity"

echo "usr_image = $usr_image, usr_verity = $usr_verity"

# Install updates to the updates directory
install -m 0644 "$TMPDIR/system/$usr_image" "$UPDATES_DIR/$usr_image"
install -m 0644 "$TMPDIR/system/$usr_verity" "$UPDATES_DIR/$usr_verity"
install -m 0644 "$TMPDIR/image/boot/EFI/Linux/avyos_${VERSION}.efi" "$UPDATES_DIR/avyos_${VERSION}.efi"

# Compress updates
xz -T0 -f -k "$UPDATES_DIR/$usr_image"
xz -T0 -f -k "$UPDATES_DIR/$usr_verity"
xz -T0 -f -k "$UPDATES_DIR/avyos_${VERSION}.efi"

# Generate sha256 checksums
(cd "$UPDATES_DIR" && sha256sum "$usr_image.xz" "$usr_verity.xz" "avyos_${VERSION}.efi.xz" > SHA256SUMS)

printf 'Published updates to %s\n' "$UPDATES_DIR"
printf 'Update URL: https://repo.avyos.dev/releases/%s/\n' "$CHANNEL"
