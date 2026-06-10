.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

QEMU_DEVICE_OPTIONS += 			\
	-device virtio-gpu-pci		\
	-device virtio-keyboard-pci	\
	-device virtio-tablet-pci	\
	-display sdl,gl=on -vga none

include ${.TOPDIR}/build/avyos.distrib.inc
