.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/libevdev

EXTRA_CONFIGURE_ARGS += \
	-D tests=disabled \
	-D tools=disabled \
	-D documentation=disabled

include ${.TOPDIR}/build/avyos.meson.inc