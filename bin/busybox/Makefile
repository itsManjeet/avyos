.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

PACKAGE_BUSYBOX_VERSION ?= 1.38.0
SOURCES = https://busybox.net/downloads/busybox-${PACKAGE_BUSYBOX_VERSION}.tar.bz2

.DISTDIR = ${SOURCESDIR}/busybox-${PACKAGE_BUSYBOX_VERSION}

BUSYBOX_MAKEARGS  = -C ${.DISTDIR} O=${.OBJDIR}
BUSYBOX_MAKEARGS += ARCH=${TARGET_ARCH} CROSS_COMPILE=${TARGET_TRIPLET}-
BUSYBOX_MAKEARGS += CC="clang --target=${TARGET_TRIPLET} --sysroot=${.SYSROOTDIR}  -fuse-ld=lld --rtlib=compiler-rt"
BUSYBOX_MAKEARGS += HOSTCC="clang"
BUSYBOX_MAKEARGS += AR="llvm-ar"
BUSYBOX_MAKEARGS += NM="llvm-nm"
BUSYBOX_MAKEARGS += LD=ld.lld
BUSYBOX_MAKEARGS += LDFLAGS=""
BUSYBOX_MAKEARGS += STRIP=llvm-strip
BUSYBOX_MAKEARGS += OBJCOPY=llvm-objcopy
BUSYBOX_MAKEARGS += OBJDUMP=llvm-objdump

LINKS = $(shell cat ${.CURDIR}/${TARGET_ARCH}.list)

all: ${.SYSROOTDIR}/usr/bin/busybox $(addprefix ${.SYSROOTDIR}/usr/bin/,$(LINKS))

clean:
	rm -rf ${.SYSROOTDIR}/usr/bin/busybox $(addprefix ${.SYSROOTDIR}/usr/bin/,$(LINKS))
	rm -rf ${.OBJDIR}

compdb:
	true

${.SYSROOTDIR}/usr/bin/busybox: ${.OBJDIR}/busybox
	install -D -m 755 $< $@

${.SYSROOTDIR}/usr/bin/%: ${.SYSROOTDIR}/usr/bin/busybox
	ln -f $< $@

${.OBJDIR}/busybox: ${.OBJDIR}/.config
	${MAKE} ${BUSYBOX_MAKEARGS} busybox

${.OBJDIR}/.config: ${.DISTDIR}/Makefile ${.CURDIR}/${TARGET_ARCH}_defconfig
	${MAKE} ${BUSYBOX_MAKEARGS} defconfig
	cd ${.OBJDIR} && KCONFIG_PATH=$@ \
		${OUTDIR}/sources/linux-${PACKAGE_LINUX_VERSION}/scripts/kconfig/merge_config.sh \
		-m $@ ${.CURDIR}/${TARGET_ARCH}_defconfig
	${MAKE} ${BUSYBOX_MAKEARGS} oldconfig

${.DISTDIR}/Makefile: fetch


include ${.TOPDIR}/build/avyos.sources.inc
