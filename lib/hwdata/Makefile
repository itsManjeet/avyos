.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

IDFILES	:= pci.ids usb.ids oui.txt iab.txt pnp.ids

TARGETS := \
	$(addprefix ${SYSTEM_PATH}/data/hwdata/,${IDFILES}) \
	${SYSTEM_PATH}/lib/pkgconfig/hwdata.pc

all: ${TARGETS}

${SYSTEM_PATH}/data/hwdata/%: ${.CURDIR}/%
	install -v -D -m 0644 $< $@

${SYSTEM_PATH}/lib/pkgconfig/%: ${.CURDIR}/%
	install -v -D -m 0644 $< $@

compile_db: