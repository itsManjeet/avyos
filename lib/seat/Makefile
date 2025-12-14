.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/seatd

EXTRA_CONFIGURE_ARGS += \
	-D libseat-builtin=enabled

include ${.TOPDIR}/build/avyos.meson.inc