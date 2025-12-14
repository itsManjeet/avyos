.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/wayland
HOST_BUILD := 1

EXTRA_CONFIGURE_ARGS += \
	-D scanner=true \
	-D libraries=false \
	-D tests=false \
	-D documentation=false \
	-D dtd_validation=false

include ${.TOPDIR}/build/avyos.meson.inc