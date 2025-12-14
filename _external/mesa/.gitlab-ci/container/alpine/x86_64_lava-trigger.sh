#!/usr/bin/env bash

# This is a ci-templates build script to generate a container for triggering LAVA jobs.

# When changing this file, you need to bump the following
# .gitlab-ci/image-tags.yml tags:
# ALPINE_X86_64_LAVA_TRIGGER_TAG

# shellcheck disable=SC1091
set -e

. .gitlab-ci/setup-test-env.sh

set -o xtrace

uncollapsed_section_start alpine_setup "Base Alpine system setup"

# Ephemeral packages (installed for this script and removed again at the end)
EPHEMERAL=(
    git
    py3-pip
)

# We only need these very basic packages to run the LAVA jobs
DEPS=(
    curl
    python3
    tar
    zstd
)

apk --no-cache add "${DEPS[@]}" "${EPHEMERAL[@]}"

pip3 install --break-system-packages -r bin/ci/requirements-lava.txt

cp -Rp .gitlab-ci/lava /
cp -Rp .gitlab-ci/bin/*_logger.py /lava
cp -Rp .gitlab-ci/common/init-stage1.sh /lava

. .gitlab-ci/container/container_pre_build.sh

############### Uninstall the build software

uncollapsed_section_switch alpine_cleanup "Cleaning up base Alpine system"

apk del "${EPHEMERAL[@]}"

. .gitlab-ci/container/container_post_build.sh

section_end alpine_cleanup
