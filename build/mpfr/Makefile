.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/mpc
HOST_BUILD := 1

EXTRA_CONFIGURE_ARGS += \
	--with-gmp=${TOOLCHAIN_PATH} \
	--with-mpfr=${TOOLCHAIN_PATH}

include ${.TOPDIR}/build/avyos.autotools.inc
