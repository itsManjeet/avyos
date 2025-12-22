include ${.TOPDIR}/build/avyos.toolchain.inc

DISTDIR 		?= ${.CURDIR}

INSTALL_ARGS	?= install
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
	--datadir=/data \
	--datarootdir=/data \
	--host=${HOST_TRIPLET} \
	--build=${TARGET_TRIPLET} \
	--target=${TARGET_TRIPLET}
DESTDIR         ?= ${SYSTEM_PATH}
endif

all: ${PRE_TARGETS} ${.OBJDIR}/.install-done ${POST_TARGETS}

${.OBJDIR}/.configure-done: ${DISTDIR}/configure
	cd ${.OBJDIR} && \
		$< ${CONFIGURE_ARGS} ${EXTRA_CONFIGURE_ARGS}
	
	@touch $@

${.OBJDIR}/.build-done: ${.OBJDIR}/.configure-done
	set -e; \
	cd ${.OBJDIR}; \
		${PRE_BUILD_COMMANDS}
	
	cd ${.OBJDIR}; ${MAKE} ${BUILD_ARGS}

	set -e; \
	cd ${.OBJDIR}; \
		${POST_BUILD_COMMANDS}

	@touch $@

${.OBJDIR}/.install-done: ${.OBJDIR}/.build-done
	cd ${.OBJDIR}; \
		${PRE_INSTALL_COMMANDS}
	
	cd ${.OBJDIR}; DESTDIR=${DESTDIR} ${MAKE} ${INSTALL_ARGS}

	cd ${.OBJDIR}; \
		${POST_INSTALL_COMMANDS}
	
	@touch $@

configure-help:  ${DISTDIR}/configure
	cd ${.OBJDIR} && \
		$< --help

clean:
	rm -rf ${.OBJDIR}/.*-done

distclean:
	rm -rf ${.OBJDIR}

compile_db:

include ${.TOPDIR}/build/avyos.sources.inc

.PHONE: all configure-help clean distclean
