.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/wayland

EXTRA_CONFIGURE_ARGS += \
	-D scanner=false \
	-D tests=false \
	-D documentation=false

include ${.TOPDIR}/build/avyos.meson.inc