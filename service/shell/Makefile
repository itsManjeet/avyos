.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SERVICE = shell@.service shell@ttyS0.service

SERVICE_TARGET = $(addprefix ${SYSTEM_PATH}/config/services.d/,${SERVICE})

all: ${SERVICE_TARGET}

${SYSTEM_PATH}/config/services.d/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@

${SYSTEM_PATH}/config/services.d/shell@ttyS0.service: ${SYSTEM_PATH}/config/services.d/shell@.service
	ln -sv $(notdir $<) $@

compile_db:

clean:
	rm -rf ${SERVICE_TARGET}