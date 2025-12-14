.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/binutils

HOST_BUILD := 1
EXTRA_CONFIGURE_ARGS += 		\
	--target=${TARGET_TRIPLET}	\
	--with-sysroot=${SYSROOT_PATH}	\
	--with-gmp=${TOOLCHAIN_PATH}	\
	--with-mpfr=${TOOLCHAIN_PATH}	\
	--with-mpc=${TOOLCHAIN_PATH}	\
	--disable-nls			\
	--disable-multilib

PRE_BUILD_COMMANDS += 			\
	${MAKE} configure-host;

include ${.TOPDIR}/build/avyos.autotools.inc
