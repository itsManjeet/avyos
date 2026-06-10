.TOPDIR ?= ..
include ${.TOPDIR}/build/avyos.defaults.inc

SUBDIR  = asciify copy crash delete
SUBDIR += driver filter find icon identity info link list
SUBDIR += mkdir mount move net notify power process read
SUBDIR += request resolution service shell system
SUBDIR += tree uevent write

ifndef (${PUREGO},1)
SUBDIR += busybox
endif

include ${.TOPDIR}/build/avyos.subdir.inc
