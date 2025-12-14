.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/zlib

export CHOST=${TARGET_TRIPLET}

CONFIGURE_ARGS := --prefix=/

include ${.TOPDIR}/build/avyos.autotools.inc