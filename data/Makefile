.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

TARGETS := $(shell find ${.CURDIR}/ -type f -not -path '*/Makefile')
TARGETS := ${TARGETS:${.CURDIR}/%=${SYSTEM_PATH}/data/%}

all: ${TARGETS}

${SYSTEM_PATH}/data/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

compile_db:

clean:
	rm -rf ${TARGETS}