#!/bin/sh

for i in apps/* ; do
    [ -f $i ] && continue
    [ -f $i/Makefile ] && continue
cat > $i/Makefile << "EOF"
.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc
include ${.TOPDIR}/apps/Makefile.inc

include ${.TOPDIR}/build/avyos.go.inc
EOF
done
