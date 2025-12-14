#!/usr/bin/env bash
# The relative paths in this file only become valid at runtime.
# shellcheck disable=SC1091
#
# When changing this file, you need to bump the following
# .gitlab-ci/image-tags.yml tags:
# DEBIAN_TEST_VIDEO_TAG

set -e

. .gitlab-ci/setup-test-env.sh

set -o xtrace

section_start debian_setup "Base Debian system setup"

export DEBIAN_FRONTEND=noninteractive

apt-get install -y gstreamer1.0-vaapi  # This interferes with systemd deps, install separately

# Ephemeral packages (installed for this script and removed again at the end)
EPHEMERAL=(
    g++
    libdrm-dev
    libva-dev
    meson
    pkgconf
)

DEPS=(
    gstreamer1.0-plugins-bad
    gstreamer1.0-plugins-base
    gstreamer1.0-plugins-good
    gstreamer1.0-plugins-ugly
    gstreamer1.0-tools
    libgstreamer1.0-0
    libva-drm2
    libva-wayland2
    libva2
)

apt-get update

apt-get install -y --no-remove --no-install-recommends \
      "${DEPS[@]}" "${EPHEMERAL[@]}" "${EXTRA_LOCAL_PACKAGES:-}"

. .gitlab-ci/container/container_pre_build.sh

section_end debian_setup

############### Build libva tests

. .gitlab-ci/container/build-va-tools.sh

############### Install Fluster

. .gitlab-ci/container/build-fluster.sh

############### Uninstall the build software

section_switch debian_cleanup "Cleaning up base Debian system"

apt-get purge -y "${EPHEMERAL[@]}"

. .gitlab-ci/container/container_post_build.sh

section_end debian_cleanup

############### Remove unused packages

. .gitlab-ci/container/strip-rootfs.sh
