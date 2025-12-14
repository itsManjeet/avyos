include ${.TOPDIR}/build/avyos.toolchain.inc

DISTDIR 		?= ${.CURDIR}

ifeq (${HOST_BUILD},1)
CONFIGURE_ARGS  ?= \
        --prefix=${TOOLCHAIN_PATH}
DESTDIR         ?= /
else
CONFIGURE_ARGS	?= \
	--prefix=/ \
	--bindir=/cmd \
	--sbindir=/cmd \
	--sysconfdir=/config \
	--libdir=/lib \
	--libexecdir=/lib \
	--datarootdir=/data \
	--datadir=/data

EXTRA_CONFIGURE_ARGS    += --cross-file ${.OBJDIR}/avyos.${TARGET_TRIPLET}-meson.txt
DESTDIR			?= ${SYSROOT_PATH}

ifeq (${GOARCH},amd64)
MESON_CPU = "x86_64"
MESON_CPU_FAMILY = "x86_64"
else
MESON_CPU = "${GOARCH}"
MESON_CPU_FAMILY = "${GOARCH}"
endif

define AVYOS_TOOLCHAIN_FILE
[binaries]
c =	'${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-gcc'
cpp = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-g++'
as = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-as'
ld = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ld'
ar = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ar'
ranlib = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ranlib'
strip = '${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-strip'
pkg-config = 'pkg-config'

[properties]
sys_root = '${SYSROOT_PATH}'
pkg_config_libdir = '${SYSROOT_PATH}/lib/pkgconfig'

[host_machine]
system = 'linux'
cpu_family = '${MESON_CPU_FAMILY}'
cpu = '${MESON_CPU}'
endian = 'little'
endef
export AVYOS_TOOLCHAIN_FILE

endif

all: ${.OBJDIR}/.install-done

${.OBJDIR}/.configure-done: ${DISTDIR}/meson.build
ifneq (${HOST_BUILD},1)
	echo "$$AVYOS_TOOLCHAIN_FILE" > ${.OBJDIR}/avyos.${TARGET_TRIPLET}-meson.txt
endif
	meson setup ${.OBJDIR} ${DISTDIR} ${CONFIGURE_ARGS} ${EXTRA_CONFIGURE_ARGS} --wrap-mode=nodownload
	
	@touch $@

${.OBJDIR}/.build-done: ${.OBJDIR}/.configure-done
	cd ${.OBJDIR}; \
		${PRE_BUILD_COMMANDS}
	
	meson compile -C ${.OBJDIR} ${BUILD_ARGS}

	cd ${.OBJDIR}; \
		${POST_BUILD_COMMANDS}

	@touch $@

${.OBJDIR}/.install-done: ${.OBJDIR}/.build-done
	cd ${.OBJDIR}; \
		${PRE_INSTALL_COMMANDS}
	
	DESTDIR=${DESTDIR} meson install -C ${.OBJDIR} ${INSTALL_ARGS}

	cd ${.OBJDIR}; \
		${POST_INSTALL_COMMANDS}
	
	@touch $@

clean:
	rm -rf ${.OBJDIR}/.*-done

distclean:
	rm -rf ${.OBJDIR}

include ${.TOPDIR}/build/avyos.sources.inc

.PHONE: all configure-help clean distclean
