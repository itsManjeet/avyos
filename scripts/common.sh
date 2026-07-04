#!/bin/bash

# Configuration and Environment Defaults
IGNITE="${IGNITE:-ignite}"
if [ "$IGNITE" = "ignite" ] && ! command -v ignite &> /dev/null; then
    if [ -f "./ignite" ]; then
        IGNITE="./ignite"
    elif [ -f "$(dirname "${BASH_SOURCE[0]}")/../ignite" ]; then
        IGNITE="$(dirname "${BASH_SOURCE[0]}")/../ignite"
    elif command -v go &> /dev/null && [ -d "$(dirname "${BASH_SOURCE[0]}")/../tools/ignite" ]; then
        echo "Building ignite Go binary..." >&2
        (cd "$(dirname "${BASH_SOURCE[0]}")/.." && go build -o ignite ./tools/ignite)
        IGNITE="./ignite"
    fi
fi
VERSION="${VERSION:-}"
CHANNEL="${CHANNEL:-}"
CACHE_PATH="${CACHE_PATH:-}"
WORKSPACE_PATH="${WORKSPACE_PATH:-}"

# Locate local.conf.yml either in the current directory or the parent directory of this script
CONF_FILE="local.conf.yml"
if [ ! -f "$CONF_FILE" ] && [ -f "$(dirname "${BASH_SOURCE[0]}")/../local.conf.yml" ]; then
    CONF_FILE="$(dirname "${BASH_SOURCE[0]}")/../local.conf.yml"
fi

if [ -f "$CONF_FILE" ]; then
    [ -z "$VERSION" ] && VERSION=$(grep -E '^version:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$CHANNEL" ] && CHANNEL=$(grep -E '^channel:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$CACHE_PATH" ] && CACHE_PATH=$(grep -E '^cache-path:' "$CONF_FILE" | awk '{print $2}' || true)
    [ -z "$WORKSPACE_PATH" ] && WORKSPACE_PATH=$(grep -E '^workspace-path:' "$CONF_FILE" | awk '{print $2}' || true)
fi

# Clean up configuration values (strip quotes, remove trailing slash from paths)
VERSION=$(echo "$VERSION" | tr -d '"'\')
CHANNEL=$(echo "$CHANNEL" | tr -d '"'\')
CACHE_PATH=$(echo "$CACHE_PATH" | tr -d '"'\' | sed 's/\/$//')
WORKSPACE_PATH=$(echo "$WORKSPACE_PATH" | tr -d '"'\' | sed 's/\/$//')

# Apply final defaults
VERSION="${VERSION:-9999}"
CHANNEL="${CHANNEL:-testing}"
CACHE_PATH="${CACHE_PATH:-out}"
WORKSPACE_PATH="${WORKSPACE_PATH:-workspaces}"
