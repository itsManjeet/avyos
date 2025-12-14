.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

ifeq (${PUREGO},1)
SUBDIR	 = sutra
else
SUBDIR	 = linux
endif

include ${.TOPDIR}/build/avyos.subdir.inc