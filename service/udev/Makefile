.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SERVICE = udev.service

POST_TARGETS += $(addprefix ${SYSROOT_PATH}/config/services.d/,${SERVICE})

include ${.TOPDIR}/build/avyos.go.inc


${SYSROOT_PATH}/config/services.d/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@
