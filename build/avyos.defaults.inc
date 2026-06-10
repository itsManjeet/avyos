ifndef .TOPDIR
$(error .TOPDIR is not defined)
endif

.TOPDIR			:= $(realpath ${.TOPDIR})
.CURDIR			:= ${CURDIR}

-include ${.TOPDIR}/config.inc

ifndef TARGET
$(error TARGET is not defined)
endif

include ${.TOPDIR}/distrib/${TARGET}/config.inc
ifndef TARGET_ARCH
$(error TARGET_ARCH is not defined)
endif

TARGET_TRIPLET	 = ${TARGET_ARCH}-linux-musl
HOST_TRIPLET	 = $(shell gcc -dumpmachine)

OUTDIR 			?= ${.TOPDIR}/out
SOURCESDIR		?= ${OUTDIR}/sources

.BUILDDIR 		 = ${OUTDIR}/${TARGET}
.OBJDIR			:= ${.CURDIR:${.TOPDIR}/%=${.BUILDDIR}/%}
.RELDIR			:= $(shell realpath --relative-to=${.TOPDIR} ${.CURDIR})

.TOOLCHAINDIR 	 = ${.BUILDDIR}/toolchain
.SYSROOTDIR		 = ${.TOOLCHAINDIR}/${TARGET_TRIPLET}/sysroot

.IMAGESDIR		 = ${.BUILDDIR}/images

export PATH		:= ${.TOOLCHAINDIR}/bin:${PATH}:/usr/sbin
export CC		:= clang
export CXX		:= clang++
export AS		:= clang
export LD		:= ld.lld
export AR		:= llvm-ar
export RANLIB	:= llvm-ranlib
export STRIP	:= llvm-strip

ifneq (${.HOSTBUILD},1)
export CFLAGS	:= --target=${TARGET_TRIPLET} --sysroot=${.SYSROOTDIR} ${TARGET_CFLAGS}
export CXXFLAGS := --target=${TARGET_TRIPLET} --sysroot=${.SYSROOTDIR} -stdlib=libc++ ${TARGET_CXXFLAGS}
export LDFLAGS	:= --target=${TARGET_TRIPLET} --sysroot=${.SYSROOTDIR} ${TARGET_LDFLAGS} -fuse-ld=lld --rtlib=compiler-rt

export PKG_CONFIG_SYSROOT_DIR	:= ${.SYSROOTDIR}
export PKG_CONFIG_LIBDIR		:= ${.SYSROOTDIR}/usr/lib/pkgconfig:${.SYSROOTDIR}/usr/share/pkgconfig
endif

ifndef RELEASEBUILD
export CFLAGS	+= -O0 -g
export CXXFLAGS	+= -O0 -g
endif

$(shell mkdir -p ${.OBJDIR})

PACKAGE_LINUX_VERSION ?= 7.0.11
PACKAGE_LINUX_URI ?= https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-${PACKAGE_LINUX_VERSION}.tar.xz

PACKAGE_LLVM_VERSION  ?= 22.1.0
PACKAGE_LLVM_URI ?= https://github.com/llvm/llvm-project/releases/download/llvmorg-${PACKAGE_LLVM_VERSION}/llvm-project-${PACKAGE_LLVM_VERSION}.src.tar.xz
