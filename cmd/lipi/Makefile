.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

POST_TARGETS	+= ${SYSTEM_PATH}/cmd/sh

include ${.TOPDIR}/build/avyos.go.inc

${SYSTEM_PATH}/cmd/sh: ${SYSTEM_PATH}/cmd/lipi
	ln -sfv lipi $@