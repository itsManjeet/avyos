.DISTDIR 	?= ${.CURDIR}

ifndef LIB
$(error LIB is not defined)
endif

INCLUDES	?=
ifndef SOURCES
SOURCES		:= lib.c
endif


.SOURCES_C	:= $(filter %.c, ${SOURCES})
.SOURCES_S	:= $(filter %.s, ${SOURCES})

.OBJECTS	+= ${.SOURCES_C:.c=.lo} ${.SOURCES_S:.s=.lo}
.OBJECTS	:= $(addprefix ${.OBJDIR}/, ${.OBJECTS})

.TARGETS 	:= ${.SYSROOTDIR}/usr/lib/lib${LIB}.a
.TARGETS 	+= ${.SYSROOTDIR}/usr/lib/lib${LIB}.so
.TARGETS 	+= $(addprefix ${.SYSROOTDIR}/usr/include/${.INCLUDEPREFIX}/, ${INCLUDES})

CFLAGS		+= -fPIC
LDFLAGS		+= -shared -Wl,-soname,lib${LIB}.so

all: ${PRE_TARGETS} ${.TARGETS} ${POST_TARGETS}

clean:
	rm -f ${.OBJECTS} ${.TARGETS}

compdb:
	true

${.OBJDIR}/%.lo: ${.DISTDIR}/%.c
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)} -c -o $@ $<
	echo -e "file=$<\noutput=$@\ncommand=${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)}"> $@.compdb

${.OBJDIR}/%.lo: ${.DISTDIR}/%.s
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)} -c -o $@ $<
	echo -e "file=$<\noutput=$@\ncommand=${CC} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} ${CFLAGS} ${CFLAGS.$(notdir $<)}"> $@.compdb

${.SYSROOTDIR}/usr/lib/lib${LIB}.so: ${.OBJECTS}
	@mkdir -p $(dir $@)
	${CC} ${LDFLAGS} -o $@ $^ ${LIBRARIES}

${.SYSROOTDIR}/usr/lib/lib${LIB}.a: ${.OBJECTS}
	@mkdir -p $(dir $@)
	${AR} rcs $@ $^

${.SYSROOTDIR}/usr/include/${.INCLUDEPREFIX}/%.h: %.h
	@mkdir -p $(dir $@)
	cp $< $@

include ${.TOPDIR}/build/avyos.sources.inc
include ${.TOPDIR}/build/avyos.compdb.inc
