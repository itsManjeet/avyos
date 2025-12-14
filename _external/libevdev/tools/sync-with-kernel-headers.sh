#!/bin/sh
#
# Syncs the repository with the input.h and input-event-codes.h headers from
# a checked out source directory.
#
# Usage:
#    sync-with-kernel-headers.sh path/to/kernel v4.12

KERNEL_TREE="$1"
GIT_DIR="$KERNEL_TREE/.git"
TAG="$2"

export GIT_DIR

if [ -z "$TAG" ] || ! [ -d "$GIT_DIR" ]; then
	echo "Usage: `basename $0` path/to/kernel tag"
	exit 1
fi
if ! [ -d .git ]; then
	echo "Run me from the top-level git tree"
	exit 1
fi


file="linux/input.h"
git cat-file -p "$TAG:include/uapi/$file" > "include/linux/linux/$(basename $file)"

file="linux/input-event-codes.h"
git cat-file -p "$TAG:include/uapi/$file" > "include/linux/linux/$(basename $file)"
git cat-file -p "$TAG:include/uapi/$file" > "include/linux/freebsd/$(basename $file)"

