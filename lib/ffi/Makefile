.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/libffi

EXTRA_CONFIGURE_ARGS += \
	--disable-exec-static-tramp \
    --disable-multi-os-directory \
    --disable-static \
    --enable-pax_emutramp

include ${.TOPDIR}/build/avyos.autotools.inc

${DISTDIR}/configure: ${DISTDIR}/configure.ac
	cd ${DISTDIR} && \
		autoreconf -fiv
