.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/limine/

LOADERS	+= limine-bios.sys

ifeq (${GOARCH},amd64)
LOADERS	+= BOOTX64.EFI limine-bios-cd.bin
LOADERS	+= limine-uefi-cd.bin
else ifeq (${GOARCH},amd64)
LOADERS	+= BOOTAA64.EFI
else
$(error Unsupported ARCH for LIMINE ${GOARCH})
endif

TARGETS	:= ${TOOLCHAIN_PATH}/bin/limine
TARGETS	+= $(addprefix ${TOOLCHAIN_PATH}/share/limine/,${LOADERS})

all: ${TARGETS}

${TOOLCHAIN_PATH}/bin/limine: ${DISTDIR}/limine.c
	${CC} ${CFLAGS} -std=c99 $< -o $@ 

${TOOLCHAIN_PATH}/share/limine/%: ${DISTDIR}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

compile_db:

clean:
	rm -rf ${TARGETS}