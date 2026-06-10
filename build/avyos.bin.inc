.DISTDIR 	?= ${.CURDIR}

ifndef BIN
ifndef TARGET
$(error BIN is not defined)
endif
endif

.TARGET 	?= ${.SYSROOTDIR}/usr/bin/${BIN}

ifndef SOURCES
SOURCES		:= main.c
endif

.SOURCES_C	:= $(filter %.c, ${SOURCES})
.SOURCES_S	:= $(filter %.s, ${SOURCES})

.OBJECTS	+= ${.SOURCES_C:.c=.o} ${.SOURCES_S:.s=.o}
.OBJECTS	:= $(addprefix ${.OBJDIR}/, ${.OBJECTS})

ifdef STATIC
LDFLAGS		+= -static
endif

ifndef NOPIE
CFLAGS		+= -fpie
endif

${.OBJDIR}/%.o: ${.DISTDIR}/%.c
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)} -c -o $@ $<
	echo -e "file=$<\noutput=$@\ncommand=${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)}"> $@.compdb

${.OBJDIR}/%.o: ${.DISTDIR}/%.s
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)} -c -o $@ $<
	echo -e "file=$<\noutput=$@\ncommand=${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)}"> $@.compdb

${.TARGET}: ${.OBJECTS}
	@mkdir -p $(dir $@)
	${CC} ${LDFLAGS} -o $@ $^ ${LIBRARIES}

all: ${PRE_TARGETS} ${.TARGET} ${POST_TARGETS}

clean:
	rm -f ${.OBJECTS} ${.TARGET}

include ${.TOPDIR}/build/avyos.sources.inc
include ${.TOPDIR}/build/avyos.compdb.inc
