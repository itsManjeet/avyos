.TOPDIR ?= .
include ${.TOPDIR}/build/avyos.defaults.inc

ifneq (${PUREGO},1)
SUBDIR	 = build lib
endif

SUBDIR	+= cmd service config data
SUBDIR	+= init
SUBDIR	+= device

include ${.TOPDIR}/build/avyos.subdir.inc