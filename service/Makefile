.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR	 = shell udev

ifneq (${PUREGO},1)
SUBDIR	+= compositor
endif

include ${.TOPDIR}/build/avyos.subdir.inc