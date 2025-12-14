.TOPDIR ?= .
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR = build lib cmd service config data init device

include ${.TOPDIR}/build/avyos.subdir.inc