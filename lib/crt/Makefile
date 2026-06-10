.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc
include ${.TOPDIR}/lib/libc/Makefile.common

CPPFLAGS += -DCRT

.OBJECTS = crt1.o crti.o crtn.o Scrt1.o rcrt1.o
.TARGETS = $(addprefix ${.SYSROOTDIR}/usr/lib/,${.OBJECTS})

${.SYSROOTDIR}/usr/lib/%.o: ${.OBJDIR}/%.o
	@mkdir -p $(dir $@)
	cp $< $@

${.OBJDIR}/%.o: ${.CURDIR}/${TARGET_ARCH}/%.s
	${CC} ${CFLAGS} ${CPPFLAGS} -c $< -o $@

${.OBJDIR}/%.o: ${.CURDIR}/%.c
	${CC} ${CFLAGS} ${CPPFLAGS} -c $< -o $@

all: ${.TARGETS}

clean:
	rm -f ${.TARGETS} $(addprefix ${.OBJDIR}/,${.OBJECTS})

compdb:
	true
