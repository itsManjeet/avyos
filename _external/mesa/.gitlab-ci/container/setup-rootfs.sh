#!/usr/bin/env bash
# shellcheck disable=SC1091 # The relative paths in this file only become valid at runtime.
# When changing this file, you need to bump the following
# .gitlab-ci/image-tags.yml tags:
# DEBIAN_TEST_GL_TAG
# DEBIAN_TEST_VIDEO_TAG
# DEBIAN_TEST_VK_TAG

set -eux -o pipefail

passwd root -d
chsh -s /bin/sh

cat > /init <<EOF
#!/bin/sh
export PS1=lava-shell:
exec sh
EOF
chmod +x  /init

# Copy timezone file and remove tzdata package
rm -rf /etc/localtime
cp /usr/share/zoneinfo/Etc/UTC /etc/localtime
