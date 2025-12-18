#!/bin/sh

labwc &
LABWC_PID=$!
sleep 1

cleanup() {
    kill -9 $LABWC_PID
}

trap cleanup EXIT

ls ${XDG_RUNTIME_DIR}/
unset DISPLAY
bwrap --bind _out/$(go env GOARCH)-linux-musl/system / \
    --dev /dev --proc /proc \
    --tmpfs /dev/shm \
    --bind ${XDG_RUNTIME_DIR} /cache/runtime \
    --setenv WAYLAND_DISPLAY wayland-0 \
    --setenv XDG_RUNTIME_DIR /cache/runtime \
    --setenv PATH /cmd \
    --setenv XDG_DATA_HOME /data \
    --setenv XDG_CONFIG_HOME /config \
    --setenv HOME / \
    $@