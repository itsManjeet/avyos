.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR += go
SUBDIR += autoconf automake libtool m4
SUBDIR += kernel-headers
SUBDIR += gmp mpfr mpc
SUBDIR += binutils gcc-static

ifeq (${CRT},musl)
SUBDIR += ${CRT}
else
$(error "unsupported crt ${CRT}")
endif

SUBDIR += gcc
SUBDIR += gperf
SUBDIR += limine
SUBDIR += wayland-scanner

include ${.TOPDIR}/build/avyos.subdir.inc
