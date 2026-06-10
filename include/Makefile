.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SOURCES = ${PACKAGE_LINUX_URI}
DISTDIR = ${SOURCESDIR}/linux-${PACKAGE_LINUX_VERSION}

LINUX_MAKE_ARGS 	:= -C ${DISTDIR} ARCH=${TARGET_ARCH} O=${.OBJDIR}

ARCH_HEADERS 		 = $(wildcard ${.TOPDIR}/arch/${TARGET_ARCH}/bits/*.h)
GENERIC_HEADERS 	 = $(wildcard ${.TOPDIR}/arch/generic/bits/*.h)
BASE_HEADERS		 = $(wildcard ${.CURDIR}/*.h ${.CURDIR}/*/*.h)

TARGETS 			 = ${.OBJDIR}/.kernel-headers-done
TARGETS 			+= ${.OBJDIR}/generated/bits/alltypes.h

TARGETS 			+= ${.SYSROOTDIR}/usr/include/bits/alltypes.h
TARGETS 			+= ${.SYSROOTDIR}/usr/include/bits/syscall.h
TARGETS				+= ${BASE_HEADERS:${.CURDIR}/%=${.SYSROOTDIR}/usr/include/%}
TARGETS				+= ${ARCH_HEADERS:${.TOPDIR}/arch/${TARGET_ARCH}/%=${.SYSROOTDIR}/usr/include/%}
TARGETS				+= ${GENERIC_HEADERS:${.TOPDIR}/arch/generic/%=${.SYSROOTDIR}/usr/include/%}


all: ${TARGETS}

clean:
	rm -rf ${.OBJDIR} ${TARGETS}

compdb:
	true

${.SYSROOTDIR}/usr/include/%: %
	@mkdir -p $(dir $@)
	cp $< $@

${.SYSROOTDIR}/usr/include/bits/%: ${.TOPDIR}/arch/${TARGET_ARCH}/bits/%
	@mkdir -p $(dir $@)
	cp $< $@

${.SYSROOTDIR}/usr/include/bits/%: ${.TOPDIR}/arch/generic/bits/%
	@mkdir -p $(dir $@)
	cp $< $@

${.OBJDIR}/.kernel-headers-done: ${DISTDIR}/Makefile
	${MAKE} ${LINUX_MAKE_ARGS} mrproper
	${MAKE} ${LINUX_MAKE_ARGS} headers
	${MAKE} ${LINUX_MAKE_ARGS} headers_install INSTALL_HDR_PATH=${.SYSROOTDIR}/usr

	@touch $@

${.SYSROOTDIR}/usr/include/bits/alltypes.h: ${.OBJDIR}/generated/bits/alltypes.h
	@mkdir -p $(dir $@)
	cp $< $@

${.SYSROOTDIR}/usr/include/bits/syscall.h: ${.OBJDIR}/generated/bits/syscall.h
	@mkdir -p $(dir $@)
	cp $< $@

${.OBJDIR}/generated/bits/alltypes.h: ${.CURDIR}/alltypes.h.sed ${.TOPDIR}/arch/${TARGET_ARCH}/bits/alltypes.h.in ${.CURDIR}/alltypes.h.in
	@mkdir -p $(dir $@)
	sed -f $+ > $@

${.OBJDIR}/generated/bits/syscall.h: ${.TOPDIR}/arch/${TARGET_ARCH}/bits/syscall.h.in
	@mkdir -p $(dir $@)
	cp $< $@
	sed -n -e s/__NR_/SYS_/p < $< >> $@


include ${.TOPDIR}/build/avyos.sources.inc
