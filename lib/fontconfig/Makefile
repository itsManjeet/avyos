.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/fontconfig

EXTRA_CONFIGURE_ARGS += \
	-D default-fonts-dirs=/data/fonts \
	-D cache-dir=/cache/fontconfig \
	-D tests=disabled

include ${.TOPDIR}/build/avyos.meson.inc