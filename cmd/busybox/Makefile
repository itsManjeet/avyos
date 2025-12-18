.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR  = ${.TOPDIR}/_external/busybox

ifeq (${GOARCH},amd64)
BUSYBOX_ARCH = x86_64
else
BUSYBOX_ARCH = ${GOARCH}
endif

BUSYBOX_MAKE_ARGS	 = -C ${DISTDIR} O=${.OBJDIR}
BUSYBOX_MAKE_ARGS	+= ARCH=${BUSYBOX_ARCH} CROSS_COMPILE=${TARGET_TRIPLET}-

MERGE_CONFIG_SH		 = ${.TOPDIR}/_external/linux/scripts/kconfig/merge_config.sh

all: ${SYSTEM_PATH}/cmd/busybox

${SYSTEM_PATH}/cmd/busybox: ${.OBJDIR}/busybox
	install -D -m 755 $< $@

${.OBJDIR}/busybox: ${.OBJDIR}/.config
	${MAKE} ${BUSYBOX_MAKE_ARGS} busybox

${.OBJDIR}/.config: ${DISTDIR}/Makefile ${.CURDIR}/${GOARCH}_defconfig
	${MAKE} ${BUSYBOX_MAKE_ARGS} defconfig
	cd ${.OBJDIR} && KCONFIG_PATH=$@ \
		${MERGE_CONFIG_SH} -m $@ ${.CURDIR}/${GOARCH}_defconfig
	${MAKE} ${BUSYBOX_MAKE_ARGS} oldconfig

clean:
	${MAKE} ${BUSYBOX_MAKE_ARGS} clean
	rm -f ${.OBJDIR}/.config

install: ${.OBJDIR}/busybox
	install -D -m 755 $< ${DESTDIR}/cmd/busybox

compile_db:

fetch_sources: