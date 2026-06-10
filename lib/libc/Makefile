.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc
include ${.TOPDIR}/lib/libc/Makefile.common
include ${.TOPDIR}/lib/libc/Makefile.sources

LIB = c

CFLAGS	+= -fno-stack-protector
CFLAGS	+= -fno-stack-clash-protection
LDFLAGS += -Wl,-e,_dlstart -nostdlib
LDFLAGS += ${.SYSROOTDIR}/usr/lib/linux/libclang_rt.builtins-${TARGET_ARCH}.a

POST_TARGETS += $(addprefix ${.SYSROOTDIR}/usr/lib/,libm.so librt.so libpthread.so libcrypt.so libutil.so libxnet.so libresolv.so libdl.so libatomic.so ld-musl-x86_64.so.1)
POST_TARGETS += ${.SYSROOTDIR}/usr/bin/ldd
POST_TARGETS += ${.SYSROOTDIR}/lib ${.SYSROOTDIR}/bin ${.SYSROOTDIR}/sbin
POST_TARGETS += ${.SYSROOTDIR}/dev ${.SYSROOTDIR}/proc ${.SYSROOTDIR}/sys ${.SYSROOTDIR}/run

include ${.TOPDIR}/build/avyos.lib.inc

${.SYSROOTDIR}/usr/lib/%: ${.SYSROOTDIR}/usr/lib/libc.so
	ln -sf libc.so $@

${.SYSROOTDIR}/usr/bin/ldd:
	@mkdir -p $(dir $@)
	ln -sf ../lib/libc.so $@

${.SYSROOTDIR}/lib:
	ln -sf usr/lib $@

${.SYSROOTDIR}/bin:
	ln -sf usr/bin $@

${.SYSROOTDIR}/usr/sbin:
	mkdir -p $@

${.SYSROOTDIR}/dev ${.SYSROOTDIR}/proc ${.SYSROOTDIR}/sys ${.SYSROOTDIR}/run:
	mkdir -p $@

${.SYSROOTDIR}/sbin: ${.SYSROOTDIR}/usr/sbin
	ln -sf usr/sbin $@
