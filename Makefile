GO ?= go
GOFLAGS ?= -tags netgo
GOARCH ?= $(shell go env GOARCH)
DEBUG ?= 0
DEVICE ?= qemu

KERNEL ?= linux

PROJECT_PATH := ${CURDIR}
CACHE_PATH ?= ${CURDIR}/_cache
DOCGEN_OUT ?= ${CACHE_PATH}/docs

-include config.inc

DEVICE_CACHE_PATH ?= ${CACHE_PATH}/${DEVICE}/${GOARCH}

SYSTEM_PATH	?= ${DEVICE_CACHE_PATH}/system
SYSTEM_IMAGE ?= ${SYSTEM_PATH}.img

INITRAMFS_PATH ?= ${DEVICE_CACHE_PATH}/initramfs
INITRAMFS_IMAGE ?= ${INITRAMFS_PATH}.img

KERNEL_IMAGE := ${CURDIR}/devices/${DEVICE}/${GOARCH}/kernel.img
KERNEL_DRIVERS_PATH ?= ${DEVICE_CACHE_PATH}/drivers

LINUX_KERNEL_VERSION ?= 7.0-rc6
KERNEL_MAKE_ARGS += LLVM=1

DISK_IMAGE ?= ${DEVICE_CACHE_PATH}/disk.img
QEMU_DEBUG_LOG ?= ${DEVICE_CACHE_PATH}/qemu-debug.log

ifeq (${GOARCH},amd64)
QEMU ?= qemu-system-x86_64
KERNEL_MAKE_ARGS += ARCH=x86_64 CROSS_COMPILE=x86_64-linux-gnu-
else ifeq (${GOARCH},arm64)
QEMU ?= qemu-system-aarch64
QEMU_ARCH_ARGS ?= -M virt -cpu cortex-a57
KERNEL_MAKE_ARGS += ARCH=aarch64 CROSS_COMPILE=aarch64-linux-gnu-
else
QEMU ?= qemu-system-${GOARCH}
endif

ifeq (${GOARCH},arm64)
KARGS ?= console=ttyAMA0,115200
else
KARGS ?= console=ttyS0 console=tty0
endif


ifeq ($(shell go env GOOS),linux)
QEMU_ACCEL ?= -accel kvm
QEMU_DISPLAY ?= -display sdl,show-cursor=on
else ifeq ($(shell go env GOOS),darwin)
QEMU_ACCEL ?= -accel hvf
QEMU_DISPLAY ?= -display cocoa
endif

ifeq (${QEMU_VNC},1)
QEMU_VNC_OPTIONS = -vnc :0
endif

DBG_PORT ?= 5037
QEMU_NET_ARGS ?= -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:${DBG_PORT}-:5037

QEMU_COMMON_ARGS ?= -smp 2 -m 2G \
	-serial mon:stdio \
	${QEMU_NET_ARGS} ${QEMU_DISPLAY} \
	-vga none ${QEMU_ACCEL} ${QEMU_VNC_OPTIONS} \
	-device virtio-gpu-pci \
	-device virtio-keyboard-pci \
	-usb -device usb-tablet

QEMU_FW_ARGS ?= -drive if=pflash,file=${CURDIR}/devices/${DEVICE}/${GOARCH}/firmware,readonly=on,format=raw \
	-drive if=pflash,file=${DEVICE_CACHE_PATH}/variables,format=raw

QEMU_DEBUG_ARGS ?= -d int,guest_errors,cpu_reset -D ${QEMU_DEBUG_LOG} \
	-no-reboot -no-shutdown -s -S

GENIMAGE_DEPS := ./tools/genimage/main.go ./tools/genimage/assets/btrfs-512m.img.gz
DOCGEN_DEPS := ./tools/docgen/main.go
RUN_EXTRA_ARGS = $(if $(filter 1,$(DEBUG)),$(QEMU_DEBUG_ARGS),)

COMMANDS = asciify copy coredump crash delete distro driver filter find identity icon info init link list mkdir mount move notify net power process read request resolution service session shell system tree uevent write
APPS = demo files notepad settings terminal
SERVICES = dbg desktop distro http login service uevent
GO_TARGETS = $(addprefix cmd/,${COMMANDS}) $(addsuffix /exec,$(addprefix apps/,${APPS})) $(addprefix services/,${SERVICES})
CONFIG_TARGETS = $(shell find config/ -type f)
DATA_TARGETS = $(shell find data/ -type f)
EXTERNAL_TARGETS =
SYSTEM_TARGETS = $(GO_TARGETS) ${EXTERNAL_TARGETS} ${CONFIG_TARGETS} ${DATA_TARGETS} $(addsuffix /manifest.json,$(addprefix apps/,${APPS})) $(addsuffix /icon.svg,$(addprefix apps/,${APPS}))

INITRAMFS_TARGETS = init

ifeq (${RELEASE},1)
GOFLAGS += -buildvcs=false -trimpath -ldflags="-s -w -buildid="
endif

all: ${DISK_IMAGE}

.PHONY: clean update-certificates run compile_db docs

clean:
	rm -rf ${SYSTEM_PATH} ${INITRAMFS_PATH}
	rm -f ${SYSTEM_IMAGE} ${INITRAMFS_IMAGE} ${DISK_IMAGE}
ifneq (${KERNEL},linux)
	rm -f ${KERNEL_IMAGE}
endif

update-certificates:
	wget https://curl.se/ca/cacert.pem -O config/certificates/ca-certificates.crt

run: ${DISK_IMAGE} ${DEVICE_CACHE_PATH}/variables
	@if [ "${DEBUG}" = "1" ]; then mkdir -p $(dir ${QEMU_DEBUG_LOG}); fi
	${QEMU} ${QEMU_ARCH_ARGS} ${QEMU_COMMON_ARGS} ${RUN_EXTRA_ARGS} ${QEMU_FW_ARGS} \
		-drive file=$<,format=raw

compile_db:
	@:> depends.${GOARCH}.inc
	@for i in ${GO_TARGETS} ; do \
		echo "${SYSTEM_PATH}/$$i: $$(find $$(go list ${GOFLAGS} -deps avyos.dev/$${i%/exec} | grep '^avyos.dev' | sed 's#^avyos.dev/#${CURDIR}/#g') -type f -name '*.go' | sort | tr '\n' ' ')" >> depends.${GOARCH}.inc; \
	done

docs: ${DOCGEN_DEPS}
	${GO} run avyos.dev/tools/docgen -out ${DOCGEN_OUT}

${CACHE_PATH}/linux-${LINUX_KERNEL_VERSION}.tar.xz:
	wget -nc https://git.kernel.org/torvalds/t/linux-${LINUX_KERNEL_VERSION}.tar.gz -O $@

${DEVICE_CACHE_PATH}/kernel/Makefile: ${CACHE_PATH}/linux-${LINUX_KERNEL_VERSION}.tar.xz
	@mkdir -p $(dir $@)
	tar -xmf $< -C ${DEVICE_CACHE_PATH}/kernel --strip-components=1

update-kernel: ${DEVICE_CACHE_PATH}/kernel/Makefile
	cd ${DEVICE_CACHE_PATH}/kernel && \
		${MAKE} ${KERNEL_MAKE_ARGS} defconfig && \
		./scripts/kconfig/merge_config.sh -m .config \
			${PROJECT_PATH}/devices/kernel.config \
			${PROJECT_PATH}/devices/${DEVICE}/${GOARCH}/kernel.config && \
		${MAKE} ${KERNEL_MAKE_ARGS} olddefconfig && \
		${MAKE} ${KERNEL_MAKE_ARGS} all

	cd ${DEVICE_CACHE_PATH}/kernel && \
		cp $$(${MAKE} ${KERNEL_MAKE_ARGS} -s image_name) ${PROJECT_PATH}/devices/${DEVICE}/${GOARCH}/kernel.img

	cd ${DEVICE_CACHE_PATH}/kernel && \
		${MAKE} ${KERNEL_MAKE_ARGS} INSTALL_MOD_PATH=${KERNEL_DRIVERS_PATH}/ INSTALL_MOD_STRIP=1 modules_install

${DEVICE_CACHE_PATH}/variables: ${CURDIR}/devices/${DEVICE}/${GOARCH}/variables
	@mkdir -p $(dir $@)
	cp -a $< $@

${DISK_IMAGE}: ${GENIMAGE_DEPS} ${KERNEL_IMAGE} ${INITRAMFS_IMAGE} ${SYSTEM_IMAGE}
	GOOS=$(shell go env GOOS) 			\
	GOARCH=$(shell go env GOARCH) 		\
	${GO} run ./tools/genimage 			\
		-target "${GOARCH}" 			\
		-kernel ${KERNEL_IMAGE} 		\
		-initrd ${INITRAMFS_IMAGE} 		\
		-rootfs ${SYSTEM_IMAGE}			\
		-kargs "${KARGS}"				\
		-limine-path ${CURDIR}/devices/${DEVICE}/${GOARCH} \
		-out $@

${SYSTEM_PATH}/apps/%/exec:
	GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build ${GOFLAGS} -o $@ $(@:${SYSTEM_PATH}/%/exec=avyos.dev/%)

${SYSTEM_PATH}/apps/%/manifest.json:
	@mkdir -p $(dir $@)
	cp $(@:${SYSTEM_PATH}/%=${CURDIR}/%) $@

${SYSTEM_PATH}/apps/%/icon.svg:
	@mkdir -p $(dir $@)
	cp $(@:${SYSTEM_PATH}/%=${CURDIR}/%) $@

${SYSTEM_PATH}/config/%: ${CURDIR}/config/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${SYSTEM_PATH}/data/%: ${CURDIR}/data/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${SYSTEM_PATH}/%:
	GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build ${GOFLAGS} -o $@ $(@:${SYSTEM_PATH}/%=avyos.dev/%)

${SYSTEM_PATH}/cmd/dlv:
	GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build ${GOFLAGS} -o $@ github.com/go-delve/delve/cmd/dlv

${SYSTEM_IMAGE}: $(addprefix ${SYSTEM_PATH}/,${SYSTEM_TARGETS})
	@rm -rf ${SYSTEM_PATH}/drivers
	[ -d ${KERNEL_DRIVERS_PATH} ] && cp -r ${KERNEL_DRIVERS_PATH} ${SYSTEM_PATH}/drivers || true
	mksquashfs ${SYSTEM_PATH} ${SYSTEM_IMAGE} -noappend -all-root -quiet

${INITRAMFS_PATH}/%: ${SYSTEM_PATH}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${INITRAMFS_PATH}/init: ${SYSTEM_PATH}/cmd/init
	@mkdir -p $(dir $@)
	cp -a $< $@

${INITRAMFS_IMAGE}: $(addprefix ${INITRAMFS_PATH}/,${INITRAMFS_TARGETS})
	(cd ${INITRAMFS_PATH}; find . -print0 | cpio --null --create --verbose --format=newc) > $@

-include depends.${GOARCH}.inc
