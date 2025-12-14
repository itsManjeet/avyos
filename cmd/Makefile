.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR	 = init lipi

ifneq (${PUREGO},1)
SUBDIR	+= busybox
endif

include ${.TOPDIR}/build/avyos.subdir.inc