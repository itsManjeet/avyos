.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

LIBRARY		:= udev
SOURCES		 = udev.c udev_list.c udev_device.c udev_monitor.c udev_enumerate.c
HEADERS		 = libudev.h

CPPFLAGS	+= -D_XOPEN_SOURCE=700
CSTD		 = 99

POST_TARGETS += ${SYSROOT_PATH}/lib/pkgconfig/libudev.pc

include ${.TOPDIR}/build/avyos.cc.inc

${SYSROOT_PATH}/lib/pkgconfig/libudev.pc: ${.CURDIR}/libudev.pc
	install -v -D -m 0644 $< $@
