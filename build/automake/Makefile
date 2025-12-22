.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/automake
HOST_BUILD := 1

PRE_TARGETS += ${.OBJDIR}/.bootstrap-done

include ${.TOPDIR}/build/avyos.autotools.inc

${.OBJDIR}/.bootstrap-done: ${DISTDIR}/bootstrap
	cd ${DISTDIR}; ./bootstrap