#!/bin/bash
set -euo pipefail

# Source common configuration
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/common.sh"

# QEMU Specific Configurations
QEMU="${QEMU:-}"
QEMU_MEMORY="${QEMU_MEMORY:-}"
QEMU_SMP="${QEMU_SMP:-}"
QEMU_VNC_HOST="${QEMU_VNC_HOST:-}"
QEMU_VNC_PORT="${QEMU_VNC_PORT:-}"
QEMU_OVMF_CODE="${QEMU_OVMF_CODE:-}"
QEMU_OVMF_VARS="${QEMU_OVMF_VARS:-}"
QEMU_EXTRA_ARGS="${QEMU_EXTRA_ARGS:-}"

# Parse local.conf.yml for QEMU settings if they are not already set in environment
CONF_FILE="local.conf.yml"
if [ ! -f "$CONF_FILE" ] && [ -f "${SCRIPT_DIR}/../local.conf.yml" ]; then
    CONF_FILE="${SCRIPT_DIR}/../local.conf.yml"
fi

if [ -f "$CONF_FILE" ]; then
    [ -z "$QEMU" ] && QEMU=$(grep -E '^qemu:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_MEMORY" ] && QEMU_MEMORY=$(grep -E '^qemu-memory:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_SMP" ] && QEMU_SMP=$(grep -E '^qemu-smp:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_VNC_HOST" ] && QEMU_VNC_HOST=$(grep -E '^qemu-vnc-host:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_VNC_PORT" ] && QEMU_VNC_PORT=$(grep -E '^qemu-vnc-port:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_OVMF_CODE" ] && QEMU_OVMF_CODE=$(grep -E '^qemu-ovmf-code:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_OVMF_VARS" ] && QEMU_OVMF_VARS=$(grep -E '^qemu-ovmf-vars:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$QEMU_EXTRA_ARGS" ] && QEMU_EXTRA_ARGS=$(grep -E '^qemu-extra-args:' "$CONF_FILE" | awk '{print $2}' || true)
fi

# Clean up QEMU configuration values (strip quotes)
QEMU=$(echo "$QEMU" | tr -d '"'\')
QEMU_MEMORY=$(echo "$QEMU_MEMORY" | tr -d '"'\')
QEMU_SMP=$(echo "$QEMU_SMP" | tr -d '"'\')
QEMU_VNC_HOST=$(echo "$QEMU_VNC_HOST" | tr -d '"'\')
QEMU_VNC_PORT=$(echo "$QEMU_VNC_PORT" | tr -d '"'\')
QEMU_OVMF_CODE=$(echo "$QEMU_OVMF_CODE" | tr -d '"'\')
QEMU_OVMF_VARS=$(echo "$QEMU_OVMF_VARS" | tr -d '"'\')
QEMU_EXTRA_ARGS=$(echo "$QEMU_EXTRA_ARGS" | tr -d '"'\')

# Apply final defaults
QEMU="${QEMU:-qemu-system-x86_64}"
QEMU_MEMORY="${QEMU_MEMORY:-4096}"
QEMU_SMP="${QEMU_SMP:-4}"
QEMU_VNC_HOST="${QEMU_VNC_HOST:-127.0.0.1}"
QEMU_VNC_PORT="${QEMU_VNC_PORT:-5901}"
QEMU_OVMF_CODE="${QEMU_OVMF_CODE:-/usr/share/OVMF/OVMF_CODE_4M.fd}"
QEMU_OVMF_VARS="${QEMU_OVMF_VARS:-/usr/share/OVMF/OVMF_VARS_4M.fd}"

# Paths derived from configurations
QEMU_DIR="${CACHE_PATH}/qemu"
QEMU_CHECKOUT="${QEMU_DIR}/installer-image"
QEMU_VARS="${QEMU_DIR}/OVMF_VARS.fd"

# Ensure disk.img exists, creating a 60G raw disk if it does not
if [ ! -f disk.img ]; then
    echo "Creating disk.img (60G)..."
    qemu-img create -f raw disk.img 60G
fi

# Build installer image element
ELEMENT="installer/image.yml"
echo "Building element ${ELEMENT}..."
"$IGNITE" build -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" "$ELEMENT"

# Clean up and checkout the build to QEMU checkout directory
echo "Checking out ${ELEMENT} to ${QEMU_CHECKOUT}..."
rm -rf "$QEMU_CHECKOUT"
"$IGNITE" checkout -cache-path "$CACHE_PATH" -workspace-path "$WORKSPACE_PATH" "$ELEMENT" "$QEMU_CHECKOUT"

# Verify dependencies
if [ ! -f "$QEMU_OVMF_CODE" ]; then
    echo "missing QEMU_OVMF_CODE=${QEMU_OVMF_CODE}" >&2
    exit 1
fi

if [ ! -f "$QEMU_OVMF_VARS" ]; then
    echo "missing QEMU_OVMF_VARS=${QEMU_OVMF_VARS}" >&2
    exit 1
fi

ISO_PATH="${QEMU_CHECKOUT}/avyos-${CHANNEL}-installer.iso"
if [ ! -f "$ISO_PATH" ]; then
    echo "missing installer ISO: ${ISO_PATH}" >&2
    exit 1
fi

# Prepare QEMU environment directory and vars file
install -d "$QEMU_DIR"
cp "$QEMU_OVMF_VARS" "$QEMU_VARS"

if ! [[ "$QEMU_VNC_PORT" =~ ^[0-9]+$ ]]; then
    echo "QEMU_VNC_PORT must be a TCP port number" >&2
    exit 1
fi

vnc_display=$((QEMU_VNC_PORT - 5900))
if [ "$vnc_display" -lt 0 ]; then
    echo "QEMU_VNC_PORT must be 5900 or higher" >&2
    exit 1
fi
VNC_DISPLAY="${QEMU_VNC_HOST}:${vnc_display}"

printf 'Booting %s with UEFI\n' "$ISO_PATH"
printf 'VNC TCP port: %s:%s\n' "$QEMU_VNC_HOST" "$QEMU_VNC_PORT"
printf 'VNC display: %s\n' "$VNC_DISPLAY"
printf 'VNC viewer examples: vncviewer %s or vncviewer %s::%s\n' \
    "$VNC_DISPLAY" "$QEMU_VNC_HOST" "$QEMU_VNC_PORT"

# Run QEMU
exec "$QEMU" \
    -machine q35,accel=kvm:tcg \
    -m "$QEMU_MEMORY" \
    -cpu Haswell \
    -smp "$QEMU_SMP" \
    -drive if=pflash,format=raw,readonly=on,file="$QEMU_OVMF_CODE" \
    -drive if=pflash,format=raw,file="$QEMU_VARS" \
    -drive if=none,id=disk0,format=raw,file=disk.img \
    -device virtio-blk-pci,drive=disk0,bootindex=1 \
    -drive if=none,id=cdrom0,media=cdrom,readonly=on,file="$ISO_PATH" \
    -device ide-cd,drive=cdrom0,bootindex=2 \
    -netdev user,id=net0 \
    -device virtio-net-pci,netdev=net0 \
    -serial mon:stdio \
    -vnc "$VNC_DISPLAY" \
    $QEMU_EXTRA_ARGS
