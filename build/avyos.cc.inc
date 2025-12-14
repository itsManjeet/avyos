include ${.TOPDIR}/build/avyos.toolchain.inc

CSTD				?= 23
CXXSTD				?= 23
SHARED				?= 1

ifdef LIBRARY
TARGET				:= ${SYSROOT_PATH}/lib/lib${LIBRARY}
ifeq (${SHARED}, 1)
TARGET				:= $(addsuffix .so,${TARGET})
else
TARGET				:= $(addsuffix .a,${TARGET})
endif
POST_TARGETS 		+= $(addprefix ${SYSROOT_PATH}/include/${HEADERS_PREFIX}/,${HEADERS})
LDFLAGS				+= "-Wl,-soname,lib${LIBRARY}.so"
else ifdef COMMAND
TARGET 				:= ${SYSROOT_PATH}/cmd/${COMMAND}
CFLAGS				+= -fPIE
CXXFLAGS			+= -fPIE
LDFLAGS				+= -fPIE -pie
else ifdef SERVICE
TARGET 				:= ${SYSROOT_PATH}/service/${SERVICE}
CFLAGS				+= -fPIE
CXXFLAGS			+= -fPIE
LDFLAGS				+= -fPIE -pie
endif

ifeq 	  (${CSTD}, 23)
CFLAGS				+= -std=c23
else ifeq (${CSTD}, 17)
CFLAGS				+= -std=c17
else ifeq (${CSTD}, 99)
CFLAGS				+= -std=c99
else ifeq (${CSTD}, 89)
CFLAGS				+= -std=c89
else
$(error "Invalid C STANDARD -std=${CSTD}")
endif

ifeq 	  (${CXXSTD}, 23)
CXXFLAGS				+= -std=c++23
else ifeq (${CXXSTD}, 20)
CXXFLAGS				+= -std=c++20
else ifeq (${CXXSTD}, 17)
CXXFLAGS				+= -std=c++17
else ifeq (${CXXSTD}, 14)
CXXFLAGS				+= -std=c++14
else ifeq (${CXXSTD}, 11)
CXXFLAGS				+= -std=c++11
else ifeq (${CXXSTD}, 98)
CXXFLAGS				+= -std=c++98
else
$(error "Invalid CXX STANDARD -std=${CXXSTD}")
endif

ifdef LIBRARIES
LIBRARIES_TARGET	 = $(addsuffix .so, $(addprefix ${SYSROOT_PATH}/lib/lib,${LIBRARIES}))
LDFLAGS				+= $(addprefix -l,${LIBRARIES})
endif

ifeq (${GNU_SOURCE}, 1)
CPPFLAGS			+= -D_GNU_SOURCE
endif

ifeq (${FREESTANDING}, 1)
CFLAGS				+= -nostdinc
CFLAGS				+= -ffreestanding
LDFLAGS				+= -nostdlib
endif

ifeq (${SHARED}, 1)
CFLAGS				+= -fPIC
ifdef LIBRARY
LDFLAGS				+= -shared
endif
else
LDFLAGS				+= -static
endif

VPATH 				+= ${.CURDIR}

SOURCES				:= $(addprefix ${.CURDIR}/,${SOURCES})
C_SOURCES			 = $(filter %.c,${SOURCES})
CXX_SOURCES			 = $(filter %.cxx,${SOURCES})
IXX_SOURCES			 = $(filter %.ixx,${SOURCES})
S_SOURCES			 = $(filter %.s,${SOURCES})

OBJECTS				+= ${C_SOURCES:.c=.o}
OBJECTS				+= ${CXX_SOURCES:.cxx=.o}
OBJECTS				+= ${S_SOURCES:.s=.o}
OBJECTS				:= ${OBJECTS:${.CURDIR}/%=${.OBJDIR}/%}

MODULES				+= ${IXX_SOURCES:.ixx=.gcm}
MODULES				:= ${MODULES:${.CURDIR}/%=${.OBJDIR}/%}

ifdef OBJECT
all: ${PRE_TARGETS} ${OBJECTS} ${POST_TARGETS}
else
all: ${PRE_TARGETS} ${TARGET} ${POST_TARGETS}
endif

${TARGET} : ${MODULES} ${OBJECTS} ${LIBRARIES_TARGET}

${.OBJDIR}/%.o: ${.CURDIR}/%.s
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} -c ${CFLAGS} ${CFLAGS.$(notdir $<)} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} $< -o $@

${.OBJDIR}/%.o: ${.CURDIR}/%.c
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CC} -c ${CFLAGS} ${CFLAGS.$(notdir $<)} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} $< -o $@

${.OBJDIR}/%.o: ${.CURDIR}/%.cxx ${MODULES}
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CXX} -c ${CXXFLAGS} -fmodules ${CXXFLAGS.$(notdir $<)} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} $< -o $@

${.OBJDIR}/%.gcm: ${.CURDIR}/%.ixx
	@mkdir -p $(dir $@)
	cd ${.OBJDIR} && ${CXX} -c ${CXXFLAGS} -fmodules ${CXXFLAGS.$(notdir $<)} ${CPPFLAGS} ${CPPFLAGS.$(notdir $<)} $< -o $@

${SYSROOT_PATH}/include/${HEADERS_PREFIX}/%.h: ${.CURDIR}/%.h
	install -D $< $@

${TARGET}:
	@mkdir -p $(dir $@)
ifeq (${CXX_SOURCES},)
	${CC} ${OBJECTS} ${LDFLAGS} -o $@
else
	${CXX} ${OBJECTS} ${MODULES} ${LDFLAGS} -o $@
endif

clean:
	rm -f ${TARGET} ${OBJECTS} ${GENERATED_FILES}

compile_db:
	@for c in ${SOURCES} ; do \
		jq --argjson obj \
		"{\"directory\": \"$(abspath .)\", \"command\": \"${CC} -c ${CFLAGS} ${CPPFLAGS} $$c\", \"file\": \"$(abspath .)/$$c\"}" \
			'. += [$$obj]' \
			${.BUILDDIR}/compile_commands.json > ${.BUILDDIR}/compile_commands.json.tmp && \
				mv ${.BUILDDIR}/compile_commands.json.tmp ${.BUILDDIR}/compile_commands.json; \
	done

.PHONY: all clean install compile_db