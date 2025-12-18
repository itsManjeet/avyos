.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/musl
DESTDIR := ${SYSTEM_PATH}

EXTRA_CONFIGURE_ARGS += \
	--disable-wrapper

include ${.TOPDIR}/build/avyos.autotools.inc
