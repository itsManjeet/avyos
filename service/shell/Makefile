.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SERVICE = shell@.service shell@ttyS0.service

SERVICE_TARGET = $(addprefix ${SYSROOT_PATH}/config/services.d/,${SERVICE})

all: ${SERVICE_TARGET}

${SYSROOT_PATH}/config/services.d/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@

${SYSROOT_PATH}/config/services.d/shell@ttyS0.service: ${SYSROOT_PATH}/config/services.d/shell@.service
	ln -sv $(notdir $<) $@