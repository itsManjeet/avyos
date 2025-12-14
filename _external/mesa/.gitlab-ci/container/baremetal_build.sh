#!/usr/bin/env bash
# shellcheck disable=SC2154 # arch is assigned in previous scripts

set -e
set -o xtrace

# Fetch the arm-built rootfs image and unpack it in our x86_64 container (saves
# network transfer, disk usage, and runtime on test jobs)

S3_PATH="https://${S3_HOST}/${S3_KERNEL_BUCKET}"

if curl -L --retry 3 -f --retry-delay 10 -s --head "${S3_PATH}/${FDO_UPSTREAM_REPO}/${LAVA_DISTRIBUTION_TAG}/lava-rootfs.tar.zst"; then
  ARTIFACTS_URL="${S3_PATH}/${FDO_UPSTREAM_REPO}/${LAVA_DISTRIBUTION_TAG}"
else
  ARTIFACTS_URL="${S3_PATH}/${CI_PROJECT_PATH}/${LAVA_DISTRIBUTION_TAG}"
fi

curl -L --retry 4 -f --retry-all-errors --retry-delay 60 \
    "${ARTIFACTS_URL}"/lava-rootfs.tar.zst -o rootfs.tar.zst
mkdir -p /rootfs-"$arch"
tar -C /rootfs-"$arch" '--exclude=./dev/*' --zstd -xf rootfs.tar.zst
rm rootfs.tar.zst

if [[ $arch == "arm64" ]]; then
    mkdir -p /baremetal-files
    pushd /baremetal-files

    curl -L --retry 4 -f --retry-all-errors --retry-delay 60 \
        -O "${KERNEL_IMAGE_BASE}"/arm64/Image
    curl -L --retry 4 -f --retry-all-errors --retry-delay 60 \
        -O "${KERNEL_IMAGE_BASE}"/arm64/Image.gz
    popd
fi
