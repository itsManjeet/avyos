.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/libinput

EXTRA_CONFIGURE_ARGS += \
	-D libwacom=false \
	-D debug-gui=false \
	-D mandir=data/man

include ${.TOPDIR}/build/avyos.meson.inc