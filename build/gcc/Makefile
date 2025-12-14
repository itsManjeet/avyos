.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/gcc

HOST_BUILD := 1

EXTRA_CONFIGURE_ARGS += 				\
	--target=${TARGET_TRIPLET}			\
	--build=${HOST_TRIPLET}				\
	--host=${HOST_TRIPLET}				\
	--libexecdir=${TOOLCHAIN_PATH}/lib 		\
	--with-sysroot=${SYSROOT_PATH}			\
	--with-local-prefix=/ 				\
	--with-native-system-header-dir="/include"	\
	--with-gmp=${TOOLCHAIN_PATH}			\
	--with-mpfr=${TOOLCHAIN_PATH}			\
	--with-mpc=${TOOLCHAIN_PATH}			\
	--enable-c99					\
	--enable-language=c,c++				\
	--disable-nls					\
	--disable-libmudflap 				\
	--disable-multilib 				\
	--disable-libmpx 				\
	--disable-libssp 				\
	--disable-libsanitizer

include ${.TOPDIR}/build/avyos.autotools.inc
