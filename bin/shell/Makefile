.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

POST_TARGETS += ${.SYSROOTDIR}/usr/bin/sh

include ${.TOPDIR}/build/avyos.go.inc

${.SYSROOTDIR}/usr/bin/sh:
	@mkdir -p $(dir $@)
	ln -sv shell $@
