#!/bin/sh


for i in services/* ; do
    if [ -d "$i" ]; then
        cat > ${i}/Makefile <<EOF
.TOPDIR ?= ../..
include \${.TOPDIR}/build/avyos.defaults.inc

include \${.TOPDIR}/build/avyos.go.inc
EOF
    fi
done
