.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR  = kernel-headers
SUBDIR += binutils gcc-static

ifeq (${CRT},musl)
SUBDIR += ${CRT}
else
$(error "unsupported crt ${CRT}")
endif

SUBDIR += gcc
SUBDIR += mtools limine

include ${.TOPDIR}/build/avyos.subdir.inc
