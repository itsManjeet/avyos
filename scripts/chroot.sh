#!/bin/sh -e

COMMAND=${@:-/bin/sh}

if [ -z "$SYSROOTDIR" ]; then
    echo "SYSROOTDIR not set, skipping chroot"
    exit 0
fi

if ! which bwrap >/dev/null; then
    echo "bwrap not found, skipping chroot"
    exit 0
fi

bwrap --bind ${SYSROOTDIR} / \
    --dev /dev --proc /proc --tmpfs /dev/shm \
    --bind ${XDG_RUNTIME_DIR} /run/user/$(id -u) \
    --setenv WAYLAND_DISPLAY ${WAYLAND_DISPLAY} \
    --setenv XDG_RUNTIME_DIR /run/user/$(id -u) \
    --setenv PATH /usr/bin:/bin:/usr/sbin:/sbin \
    --setenv HOME / $COMMAND
