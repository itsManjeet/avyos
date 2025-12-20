ifndef .TOPDIR
$(error .TOPDIR is not defined)
endif

.TOPDIR					:= $(realpath ${.TOPDIR})
.CURDIR					:= ${CURDIR}

-include ${.TOPDIR}/config.inc

GOARCH					?= $(shell go env GOARCH)
GOOS					:= linux
CRT						?= musl

ifeq (${CRT},musl)
else
$(error unsupported CRT = '${CRT}')
endif

TARGET_TRIPLET			:= ${GOARCH}-${GOOS}-${CRT}
HOST_TRIPLET			:= $(shell gcc -dumpmachine)

CACHE_PATH              ?= ${.TOPDIR}/_out
.BUILDDIR				:= ${CACHE_PATH}/${TARGET_TRIPLET}

.OBJDIR					:= ${.CURDIR:${.TOPDIR}/%=${.BUILDDIR}/%}
.RELDIR					:= $(shell realpath --relative-to=${.TOPDIR} ${.CURDIR})

TOOLCHAIN_PATH 			:= ${.BUILDDIR}/toolchain
SYSTEM_PATH				:= ${.BUILDDIR}/system
IMAGES_PATH				:= ${.BUILDDIR}/images

SOURCES_PATH			:= ${CACHE_PATH}/sources

export PATH				:= ${TOOLCHAIN_PATH}/bin:${PATH}:/usr/sbin

$(shell mkdir -p ${.OBJDIR})
