.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR  = autoconf automake

SUBDIR += kernel-headers
SUBDIR += gmp mpfr mpc
SUBDIR += binutils gcc-static

ifeq (${CRT},musl)
SUBDIR += ${CRT}
else
$(error "unsupported crt ${CRT}")
endif

SUBDIR += gcc
SUBDIR += limine
SUBDIR += wayland-scanner

include ${.TOPDIR}/build/avyos.subdir.inc
