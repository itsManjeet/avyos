.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/wbg

EXTRA_CONFIGURE_ARGS += \
	-D png=enabled \
	-D jpeg=disabled \
	-D jxl=disabled \
	-D webp=disabled \
	-D svg=true

include ${.TOPDIR}/build/avyos.meson.inc