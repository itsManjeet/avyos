.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SERVICE = connect.service

POST_TARGETS += $(addprefix ${SYSTEM_PATH}/config/services.d/,${SERVICE})

include ${.TOPDIR}/build/avyos.go.inc

${SYSTEM_PATH}/config/services.d/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@