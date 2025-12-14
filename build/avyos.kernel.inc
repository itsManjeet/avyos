KERNEL_SOURCE ?= ${.TOPDIR}/_external/linux

ifeq (${GOARCH},amd64)
KERNEL_ARCH := x86_64
else
KERNEL_ARCH	:= $(GOARCH)
endif

KERNEL_MAKE_ARGS := -C ${KERNEL_SOURCE} O=${.OBJDIR}
KERNEL_MAKE_ARGS += ARCH=${KERNEL_ARCH} CROSS_COMPILE=${TARGET_TRIPLET}-
KERNEL_MAKE_ARGS += KCONFIG_PATH=${.OBJDIR}/.config

all: ${.OBJDIR}/.install-done

${.OBJDIR}/.install-done: ${.OBJDIR}/.build-done
	cp ${.OBJDIR}/$(shell make ${KERNEL_MAKE_ARGS} -s image_name) ${IMAGES_PATH}/kernel.img
	${MAKE} ${KERNEL_MAKE_ARGS} INSTALL_MOD_PATH=${SYSROOT_PATH}/ INSTALL_MOD_STRIP=1 modules_install
	touch $@

${.OBJDIR}/.configure-done: ${KERNEL_SOURCE}/Makefile ${KERNEL_CONFIG}
	${MAKE} ${KERNEL_MAKE_ARGS} defconfig
	cd ${.OBJDIR} && KCONFIG_PATH=${.OBJDIR}/.config \
		${KERNEL_SOURCE}/scripts/kconfig/merge_config.sh -m ${.OBJDIR}/.config \
			${KERNEL_CONFIG}
	${MAKE} ${KERNEL_MAKE_ARGS} olddefconfig
	touch $@

${.OBJDIR}/.build-done: ${.OBJDIR}/.configure-done
	@mkdir -p ${IMAGES_PATH}
	${MAKE} ${KERNEL_MAKE_ARGS} bzImage modules
	touch $@

reset-build:
	rm -rf ${.OBJDIR}/.*-done ${.OBJDIR}/.config