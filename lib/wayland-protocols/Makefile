.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/wayland-protocols

EXTRA_CONFIGURE_ARGS += \
	-D tests=false

include ${.TOPDIR}/build/avyos.meson.inc