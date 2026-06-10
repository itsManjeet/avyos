include ${.TOPDIR}/build/avyos.defaults.inc

GO 				?= go
GOFLAGS 		?=
CGO_ENABLED 	?= 0

ifeq (${CGO_ENABLED},1)
include ${.TOPDIR}/build/avyos.toolchain.inc

CGO_CFLAGS		+= --sysroot=${.SYSROOTDIR}
CGO_LDFLAGS		+= --sysroot=${.SYSROOTDIR}
endif

PACKAGE			?= avyos.dev/${.RELDIR}

ifeq (${LIBRARY},1)
.TARGET			?= ${.OBJDIR}/lib$(notdir ${.RELDIR}).so
CGO_ENABLED 	:= 1
GOFLAGS			+= -buildmode=c-shared
POST_TARGETS	+= ${.SYSROOTDIR}/usr/$(dir ${.RELDIR})/lib$(notdir ${.RELDIR}).so
POST_TARGETS	+= ${.SYSROOTDIR}/usr/include/$(notdir ${.RELDIR}).h
else
.TARGET			?= ${.SYSROOTDIR}/usr/${.RELDIR}
endif

all: ${PRE_TARGETS} ${.TARGET} ${POST_TARGETS}

${.TARGET}:
ifeq (${CGO_ENABLED},1)
	CGO_CFLAGS="${CGO_CFLAGS}" \
	CGO_LDFLAGS="${CGO_LDFLAGS}" \
	CGO_ENABLED="${CGO_ENABLED}" \
	GOOS="${GOOS}" \
	GOARCH="${GOARCH}" \
	${GO} build ${GOFLAGS} -o $@ ${PACKAGE}
else
	CGO_ENABLED="${CGO_ENABLED}" GOOS="${GOOS}" GOARCH="${GOARCH}" ${GO} build ${GOFLAGS} -o $@ ${PACKAGE}
endif


clean:
	rm -rf ${.TARGET}

compdb:
	echo "${.TARGET}: $$(find $$(go -C ${.TOPDIR} list ${GOFLAGS} -deps ${PACKAGE} | grep '^avyos.dev' | sed 's#^avyos.dev/#${.TOPDIR}/#g') -type f -name '*.go' | sort | tr '\n' ' ')" > ${.OBJDIR}/depends.inc


.PHONY: all clean

-include ${.OBJDIR}/depends.inc

${.SYSROOTDIR}/usr/$(dir ${.RELDIR})/lib$(notdir ${.RELDIR}).so: ${.TARGET}
	@mkdir -p $(dir $@)
	cp $< $@

${.SYSROOTDIR}/usr/include/$(notdir ${.RELDIR}).h: ${.TARGET} ${.OBJDIR}/lib$(notdir ${.RELDIR}).h
	@mkdir -p $(dir $@)
	cp ${.OBJDIR}/lib$(notdir ${.RELDIR}).h $@
