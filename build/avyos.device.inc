DISK_IMAGE		?= ${IMAGES_PATH}/avyos-${GOARCH}.img
DISK_SIZE		?= 5G

INITRD_FILES	+= init
INITRD_PATH		?= ${.OBJDIR}/initrd

ifeq (${KERNEL},sutra)
PROTOCOL		:= limine
else ifeq (${KERNEL},linux)
PROTOCOL		:= linux
endif

ifeq (${GOARCH},amd64)
QEMU			?= qemu-system-x86_64
else
QEMU			?= qemu-system-${GOARCH}
endif

QEMU_FLAGS		?= -smp 2 -m 2G -enable-kvm

all: ${DISK_IMAGE}

run: ${DISK_IMAGE}
	${QEMU} ${QEMU_FLAGS} \
		-drive file=${DISK_IMAGE},format=raw

${DISK_IMAGE}: ${.TOPDIR}/scripts/genimage.sh ${IMAGES_PATH}/initrd.img ${IMAGES_PATH}/kernel.img
	${.TOPDIR}/scripts/genimage.sh 					\
		-disk-size	${DISK_SIZE} 					\
		-kernel ${IMAGES_PATH}/kernel.img 			\
		-initrd ${IMAGES_PATH}/initrd.img 			\
		-rootfs ${SYSROOT_PATH}						\
		-protocol ${PROTOCOL}						\
		-limine-path ${TOOLCHAIN_PATH}/share/limine \
		-out $@

${INITRD_PATH}/%: ${SYSROOT_PATH}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${INITRD_PATH}/init: ${SYSROOT_PATH}/cmd/init
	@mkdir -p $(dir $@)
	cp -a $< $@

${IMAGES_PATH}/initrd.img: $(addprefix ${INITRD_PATH}/,${INITRD_FILES})
	cd ${INITRD_PATH} && find . -print0 | cpio --null --create --verbose --format=newc > $@

.PHONY: ${DISK_IMAGE}

