.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

GOFLAGS := \
	-gcflags=avyos.dev/init/sutra=-std \
	-ldflags=" \
		-linkmode internal \
		-E _start \
		-T 0x1FE000 \
		-R 0x1000 \
	"
TARGET := ${IMAGES_PATH}/kernel.img

include ${.TOPDIR}/build/avyos.go.inc