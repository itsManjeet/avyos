.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/linux

ifeq (${GOARCH},amd64)
KERNEL_ARCH := x86_64
else
KERNEL_ARCH := ${GOARCH}
endif

KERNEL_BUILD_ARGS := -C ${DISTDIR} ARCH=${KERNEL_ARCH} O=${.OBJDIR}

all: ${.OBJDIR}/.build-done

${.OBJDIR}/.build-done: ${DISTDIR}/Makefile
	${MAKE} ${KERNEL_BUILD_ARGS} mrproper
	${MAKE} ${KERNEL_BUILD_ARGS} headers
	${MAKE} ${KERNEL_BUILD_ARGS} headers_install INSTALL_HDR_PATH=${SYSTEM_PATH}
	
	@touch $@

compile_db:

clean:
	rm -rf ${.OBJDIR}/.*-done