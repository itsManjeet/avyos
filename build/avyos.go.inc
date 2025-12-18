include ${.TOPDIR}/build/avyos.defaults.inc

GO 		?= go
GOFLAGS ?= 
CGO_ENABLED ?= 0

ifeq (${CGO_ENABLED},1)
include ${.TOPDIR}/build/avyos.toolchain.inc

export CGO_CFLAGS	= --sysroot=${SYSTEM_PATH}
export CGO_LDFLAGS	= --sysroot=${SYSTEM_PATH}
endif

PACKAGE ?= avyos.dev/${.RELDIR}
TARGET	?= ${SYSTEM_PATH}/${.RELDIR}

all: ${PRE_TARGETS} ${TARGET} ${POST_TARGETS}

${TARGET}:
	CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} ${GO} build ${GOFLAGS} -o $@ ${PACKAGE}


clean:
	rm -rf ${TARGET}

compile_db:
	echo "${TARGET}: $$(find $$(go -C ${.TOPDIR} list ${GOFLAGS} -deps ${PACKAGE} | grep '^avyos.dev' | sed 's#^avyos.dev/#${.TOPDIR}/#g') -type f -name '*.go' | sort | tr '\n' ' ')" > ${.OBJDIR}/depends.inc


.PHONY: all clean

-include ${.OBJDIR}/depends.inc