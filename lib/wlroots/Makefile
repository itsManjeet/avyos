.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/wlroots

EXTRA_CONFIGURE_ARGS += \
	-D examples=false \
	-D renderers=gles2 \
	-D backends=drm,libinput \
	-D allocators=gbm

include ${.TOPDIR}/build/avyos.meson.inc