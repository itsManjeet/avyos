#!/bin/sh

VERSION='9999'
CHANNEL='devel'
CACHE_PATH="$PWD/out"

while [ $# -gt 0 ] ; do
    case "$1" in
        -version)
            VERSION="$2"
            shift
            ;;
        -cache-path)
            CACHE_PATH="$2"
            shift
            ;;
        -channel)
            CHANNEL="$2"
            shift
            ;;
        -*)
            echo "Usage: $0 invalid args $1"
            exit 1
            ;;
    esac
    shift
done

cat <<EOF > local.conf.yml
arch: x86_64
cache-path: ${CACHE_PATH}
workspace-path: workspaces

version: ${VERSION}
channel: ${CHANNEL}

workspace-push: false
workspace-message: rlxos workspace update
EOF
