.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/libinput

EXTRA_CONFIGURE_ARGS += \
	-D libwacom=false \
	-D debug-gui=false

include ${.TOPDIR}/build/avyos.meson.inc