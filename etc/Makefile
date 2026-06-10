.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTRIB_TARGETS := $(shell find ${.TOPDIR}/distrib/${TARGET}/etc/ -type f)
DISTRIB_TARGETS := ${DISTRIB_TARGETS:${.TOPDIR}/distrib/${TARGET}/etc/%=${.SYSROOTDIR}/usr/share/factory/etc/%}

TARGETS := $(shell find ${.CURDIR}/ -type f)
TARGETS := ${DISTRIB_TARGETS} ${TARGETS:${.CURDIR}/%=${.SYSROOTDIR}/usr/share/factory/etc/%}

${.SYSROOTDIR}/usr/share/factory/etc/%: ${.TOPDIR}/distrib/${TARGET}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${.SYSROOTDIR}/usr/share/factory/etc/%: ${.CURDIR}/${TARGET_ARCH}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${.SYSROOTDIR}/usr/share/factory/etc/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

all: ${TARGETS}

clean:
	rm -rf ${TARGETS}

compdb:
	true

include ${.TOPDIR}/build/avyos.sources.inc
