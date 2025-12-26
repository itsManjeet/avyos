include ${.TOPDIR}/build/avyos.defaults.inc

GO 		?= go
GOFLAGS ?= 
CGO_ENABLED ?= 0

ifeq (${GRAPHICS},1)
GOFLAGS 	+= --tags wayland,egl,gles2
CGO_ENABLED	:= 1
endif

ifeq (${CGO_ENABLED},1)
include ${.TOPDIR}/build/avyos.toolchain.inc

CGO_CFLAGS	+= --sysroot=${SYSTEM_PATH}
CGO_LDFLAGS	+= --sysroot=${SYSTEM_PATH}
endif

PACKAGE ?= avyos.dev/${.RELDIR}

ifeq (${LIBRARY},1)
TARGET	?= ${.OBJDIR}/lib$(notdir ${.RELDIR}).so
CGO_ENABLED := 1
GOFLAGS	+= \
	-buildmode=c-shared
POST_TARGETS	+= ${SYSTEM_PATH}/$(dir ${.RELDIR})/lib$(notdir ${.RELDIR}).so
POST_TARGETS	+= ${SYSTEM_PATH}/include/$(notdir ${.RELDIR}).h
else
TARGET	?= ${SYSTEM_PATH}/${.RELDIR}
endif

all: ${PRE_TARGETS} ${TARGET} ${POST_TARGETS}

${TARGET}:
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
	rm -rf ${TARGET}

compile_db:
	echo "${TARGET}: $$(find $$(go -C ${.TOPDIR} list ${GOFLAGS} -deps ${PACKAGE} | grep '^avyos.dev' | sed 's#^avyos.dev/#${.TOPDIR}/#g') -type f -name '*.go' | sort | tr '\n' ' ')" > ${.OBJDIR}/depends.inc


.PHONY: all clean

-include ${.OBJDIR}/depends.inc

${SYSTEM_PATH}/$(dir ${.RELDIR})/lib$(notdir ${.RELDIR}).so: ${TARGET}
	@mkdir -p $(dir $@)
	cp $< $@
${SYSTEM_PATH}/include/$(notdir ${.RELDIR}).h: ${TARGET} ${.OBJDIR}/lib$(notdir ${.RELDIR}).h
	@mkdir -p $(dir $@)
	cp ${.OBJDIR}/lib$(notdir ${.RELDIR}).h $@