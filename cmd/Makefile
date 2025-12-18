.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR	 = init lipi udev connect service

ifneq (${PUREGO},1)
SUBDIR	+= busybox
endif

include ${.TOPDIR}/build/avyos.subdir.inc