.TOPDIR ?= .
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR = tools include lib libexec bin sbin apps etc share distrib

include ${.TOPDIR}/build/avyos.subdir.inc
