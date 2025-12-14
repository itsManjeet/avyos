.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

TARGETS := $(shell find ${.CURDIR}/ -type f -not -path '*/Makefile')
TARGETS := ${TARGETS:${.CURDIR}/%=${SYSROOT_PATH}/config/%}

all: ${TARGETS}

${SYSROOT_PATH}/config/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@