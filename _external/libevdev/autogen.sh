#!/bin/sh

test -n "$srcdir" || srcdir=`dirname "$0"`
test -n "$srcdir" || srcdir=.

olddir=`pwd`

cd "$srcdir"
autoreconf -fvi || exit $?

cd "$olddir"

git config --local --get format.subjectPrefix >/dev/null 2>&1 ||
    git config --local format.subjectPrefix "PATCH libevdev"

test -n "$NOCONFIGURE" || exec "$srcdir"/configure "$@"
