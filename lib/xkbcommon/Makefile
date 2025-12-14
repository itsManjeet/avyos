.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/libxkbcommon

EXTRA_CONFIGURE_ARGS += \
	-D enable-x11=false

include ${.TOPDIR}/build/avyos.meson.inc