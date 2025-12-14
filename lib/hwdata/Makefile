.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

IDFILES	:= pci.ids usb.ids oui.txt iab.txt pnp.ids

TARGETS := \
	$(addprefix ${SYSROOT_PATH}/data/hwdata/,${IDFILES}) \
	${SYSROOT_PATH}/lib/pkgconfig/hwdata.pc

all: ${TARGETS}

${SYSROOT_PATH}/data/hwdata/%: ${.CURDIR}/%
	install -v -D -m 0644 $< $@

${SYSROOT_PATH}/lib/pkgconfig/%: ${.CURDIR}/%
	install -v -D -m 0644 $< $@
