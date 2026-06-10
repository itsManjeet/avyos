.TOPDIR ?= ../..

.HOSTBUILD := 1

include ${.TOPDIR}/build/avyos.defaults.inc

PACKAGE_LIMINE_VERSION = 12.3.3
SOURCES = https://github.com/Limine-Bootloader/Limine/releases/download/v${PACKAGE_LIMINE_VERSION}/limine-binary.tar.gz

.DISTDIR = ${SOURCESDIR}/limine-binary

ifeq (${TARGET_ARCH},x86_64)
.LOADERS		+= BOOTX64.EFI limine-bios.sys limine-bios-cd.bin
.LOADERS		+= limine-uefi-cd.bin
else ifeq (${TARGET_ARCH},aarch64)
.LOADERS		+= BOOTAA64.EFI
else
$(error Unsupported target architecture: ${TARGET_ARCH})
endif

.TARGETS := ${.TOOLCHAINDIR}/bin/limine
.TARGETS += $(addprefix ${.TOOLCHAINDIR}/share/limine/,$(.LOADERS))

all: ${.TARGETS}

clean:
	rm -f ${.TARGETS}

compdb:
	true

${.TOOLCHAINDIR}/bin/limine: ${.DISTDIR}/limine.c
	@mkdir -p $(dir $@)
	${CC} ${HOST_CFLAGS} -std=c99 $< -o $@

${.TOOLCHAINDIR}/share/limine/%: ${.DISTDIR}/%
	@mkdir -p $(dir $@)
	cp $< $@

include ${.TOPDIR}/build/avyos.sources.inc
