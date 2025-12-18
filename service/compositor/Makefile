.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SERVICE		 = compositor
SOURCES		 = dwl.c util.c

CPPFLAGS	+= -DVERSION=\"0.0.1\"
CPPFLAGS	+= -DWLR_USE_UNSTABLE -D_POSIX_C_SOURCE=200809L -I ${.CURDIR} -I ${.OBJDIR}
CPPFLAGS	+= -I ${SYSTEM_PATH}/include/wlroots-0.20
CPPFLAGS	+= -I ${SYSTEM_PATH}/include/pixman-1

LIBRARIES	+= wlroots-0.20
LIBRARIES	+= wayland-server
LIBRARIES	+= xkbcommon
LIBRARIES	+= input
LIBRARIES	+= pixman-1
LIBRARIES	+= fcft

PRE_TARGETS	+= ${.OBJDIR}/cursor-shape-v1-protocol.h
PRE_TARGETS	+= ${.OBJDIR}/pointer-constraints-unstable-v1-protocol.h
PRE_TARGETS	+= ${.OBJDIR}/wlr-layer-shell-unstable-v1-protocol.h
PRE_TARGETS	+= ${.OBJDIR}/wlr-output-power-management-unstable-v1-protocol.h
PRE_TARGETS	+= ${.OBJDIR}/xdg-shell-protocol.h

POST_TARGETS += $(addprefix ${SYSTEM_PATH}/config/services.d/,${SERVICE}@.service)
POST_TARGETS += $(addprefix ${SYSTEM_PATH}/config/services.d/,${SERVICE}@tty0.service)

include ${.TOPDIR}/build/avyos.cc.inc

${.OBJDIR}/cursor-shape-v1-protocol.h:
	wayland-scanner enum-header \
		${SYSTEM_PATH}/data/wayland-protocols/staging/cursor-shape/cursor-shape-v1.xml $@
${.OBJDIR}/pointer-constraints-unstable-v1-protocol.h:
	wayland-scanner enum-header \
		${SYSTEM_PATH}/data/wayland-protocols/unstable/pointer-constraints/pointer-constraints-unstable-v1.xml $@
${.OBJDIR}/wlr-layer-shell-unstable-v1-protocol.h:
	wayland-scanner enum-header \
		protocols/wlr-layer-shell-unstable-v1.xml $@
${.OBJDIR}/wlr-output-power-management-unstable-v1-protocol.h:
	wayland-scanner server-header \
		protocols/wlr-output-power-management-unstable-v1.xml $@
${.OBJDIR}/xdg-shell-protocol.h:
	wayland-scanner server-header \
		${SYSTEM_PATH}/data/wayland-protocols/stable/xdg-shell/xdg-shell.xml $@

${SYSTEM_PATH}/config/services.d/%: ${.CURDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@

${SYSTEM_PATH}/config/services.d/compositor@tty0.service: ${SYSTEM_PATH}/config/services.d/compositor@.service
	ln -sfv $(notdir $<) $@