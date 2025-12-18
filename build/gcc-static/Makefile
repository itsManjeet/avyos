.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/gcc

HOST_BUILD := 1

EXTRA_CONFIGURE_ARGS += 					\
	--target=${TARGET_TRIPLET}				\
	--build=${HOST_TRIPLET}					\
	--host=${HOST_TRIPLET}					\
	--libexecdir=${TOOLCHAIN_PATH}/lib 			\
	--with-sysroot=${SYSTEM_PATH}				\
	--with-local-prefix=/ 					\
	--with-gmp=${TOOLCHAIN_PATH}				\
	--with-mpfr=${TOOLCHAIN_PATH}				\
	--with-mpc=${TOOLCHAIN_PATH}				\
	--with-newlib						\
	--enable-languages=c					\
	--disable-nls						\
	--disable-shared					\
	--disable-multilib					\
	--disable-decimal-float 				\
	--disable-libgomp 					\
	--disable-libmudflap 					\
	--disable-libssp 					\
	--disable-libatomic 					\
	--disable-libquadmath 					\
	--disable-threads 					\

BUILD_ARGS = all-gcc all-target-libgcc
INSTALL_ARGS = install-gcc install-target-libgcc

include ${.TOPDIR}/build/avyos.autotools.inc
