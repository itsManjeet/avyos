.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

TARGETS := $(shell find ${.CURDIR}/ -type f -not -path '*/Makefile')
TARGETS := ${TARGETS:${.CURDIR}/%=${SYSTEM_PATH}/config/%}

all: ${TARGETS}

${SYSTEM_PATH}/config/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

compile_db:

clean:
	rm -rf ${TARGETS}