.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTRIB_TARGETS := $(shell find ${.TOPDIR}/distrib/${TARGET}/share/ -type f)
DISTRIB_TARGETS := ${DISTRIB_TARGETS:${.TOPDIR}/distrib/${TARGET}/share/%=${.SYSROOTDIR}/usr/share/%}

TARGETS := $(shell find ${.CURDIR}/ -type f)
TARGETS := ${DISTRIB_TARGETS} ${TARGETS:${.CURDIR}/%=${.SYSROOTDIR}/usr/share/%}

${.SYSROOTDIR}/usr/share/%: ${.TOPDIR}/distrib/${TARGET}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${.SYSROOTDIR}/usr/share/%: ${.CURDIR}/${TARGET_ARCH}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${.SYSROOTDIR}/usr/share/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

all: ${TARGETS}

clean:
	rm -rf ${TARGETS}

compdb:
	true

include ${.TOPDIR}/build/avyos.sources.inc
