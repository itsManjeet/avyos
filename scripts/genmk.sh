#!/bin/sh

for i in apps/* ; do
    [ -f $i ] && continue
    [ -f $i/Makefile ] && continue
cat > $i/Makefile << "EOF"
.TOPDIR ?= ../..
include ${.TOPDIR}/build/rlxos.defaults.inc
include ${.TOPDIR}/apps/Makefile.inc

include ${.TOPDIR}/build/rlxos.go.inc
EOF
done
