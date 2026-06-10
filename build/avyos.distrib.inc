INITRAMFSDIR		:= ${.OBJDIR}/initramfs
INITRAMFS_TARGETS	:= $(addprefix ${INITRAMFSDIR}/, $(shell cat ${.TOPDIR}/distrib/initramfs.list) $(shell cat ${.TOPDIR}/distrib/${TARGET}/initramfs.list))

.DISTDIR ?= ${SOURCESDIR}/linux-${PACKAGE_LINUX_VERSION}

LINUX_MAKEARGS	+= -C ${.DISTDIR} O=${.OBJDIR}
LINUX_MAKEARGS	+= ARCH=${TARGET_ARCH} CROSS_COMPILE=${TARGET_TRIPLET}- LLVM=1
LINUX_MAKEARGS	+= KCONFIG_PATH=${.OBJDIR}/.config

QEMU_DEVICE_MEMORY 		?= 2G
QEMU_DEVICE_CPU_CORE	?= 2

.PHONY: all clean compdb fetch ${.IMAGESDIR}/disk.img ${.TOPDIR}/scripts/genimage.sh ${.IMAGESDIR}/usr.img ${.IMAGESDIR}/initramfs.img

all: ${.IMAGESDIR}/disk.img

clean:
	rm -rf ${.OBJDIR}/.avyos-*-done ${.OBJDIR}/.config

compdb:
	true

run:
	qemu-system-${TARGET_ARCH}                              \
	    -drive file=${.IMAGESDIR}/disk.img,format=raw       \
		-m ${QEMU_DEVICE_MEMORY}                            \
		-serial mon:stdio                                   \
		-smp ${QEMU_DEVICE_CPU_CORE} ${QEMU_DEVICE_OPTIONS}

${.IMAGESDIR}/disk.img: ${.TOPDIR}/scripts/genimage.sh ${.IMAGESDIR}/usr.img ${.IMAGESDIR}/kernel.img ${.IMAGESDIR}/initramfs.img
	${.TOPDIR}/scripts/genimage.sh                      \
		-out            $@.temp                         \
		-arch           ${TARGET_ARCH}                  \
		-usr            ${.IMAGESDIR}/usr.img           \
		-initrd         ${.IMAGESDIR}/initramfs.img     \
		-kernel         ${.IMAGESDIR}/kernel.img        \
		-limine-path    ${.TOOLCHAINDIR}/share/limine
	mv $@.temp $@

${.IMAGESDIR}/usr.img: ${.IMAGESDIR}/kernel.img
	mksquashfs ${.SYSROOTDIR}/usr ${.IMAGESDIR}/usr.img -noappend -all-root

${INITRAMFSDIR}/%: ${.SYSROOTDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${.IMAGESDIR}/initramfs.img: ${.IMAGESDIR}/usr.img ${INITRAMFS_TARGETS}
	@mkdir -p ${INITRAMFSDIR}
	mkdir -p ${INITRAMFSDIR}/{dev,proc,sys,run,newroot}
	ln -sf usr/lib ${INITRAMFSDIR}/lib
	ln -sf usr/bin ${INITRAMFSDIR}/bin
	ln -sf usr/sbin ${INITRAMFSDIR}/sbin
	ln -sf usr/sbin/init ${INITRAMFSDIR}/init
	cd ${INITRAMFSDIR} && find . -print0 | cpio --null --create --verbose --format=newc > $@


${.IMAGESDIR}/kernel.img: ${.OBJDIR}/.avyos-compile-done
	@mkdir -p ${.IMAGESDIR}
	KERNEL_IMAGE="$$(${MAKE} ${LINUX_MAKEARGS} --no-print-directory -s image_name)" && \
		cp ${.OBJDIR}/$$KERNEL_IMAGE ${.IMAGESDIR}/kernel.img \

	${MAKE} ${LINUX_MAKEARGS} INSTALL_MOD_PATH=${.SYSROOTDIR}/usr INSTALL_MOD_STRIP=1 modules_install

	@touch $@

${.OBJDIR}/.avyos-compile-done: ${.OBJDIR}/.avyos-configure-done
	${MAKE} ${LINUX_MAKEARGS}
	@touch $@

${.OBJDIR}/.avyos-configure-done: ${.DISTDIR}/Makefile ${.TOPDIR}/distrib/${TARGET}/kernel.config ${.TOPDIR}/distrib/kernel.config
	${MAKE} ${LINUX_MAKEARGS} defconfig
	cd ${.OBJDIR} && KCONFIG_PATH=${.OBJDIR}/.config \
		${.DISTDIR}/scripts/kconfig/merge_config.sh -m ${.OBJDIR}/.config \
			${.TOPDIR}/distrib/kernel.config ${.TOPDIR}/distrib/${TARGET}/kernel.config
	${MAKE} ${LINUX_MAKEARGS} olddefconfig
	@touch $@

${.DISTDIR}/Makefile: fetch

include ${.TOPDIR}/build/avyos.sources.inc
