.DISTDIR 				?= ${.CURDIR}

ifeq (${.HOSTBUILD},1)
.CONFIGURE_ARGS			?= -DCMAKE_INSTALL_PREFIX=${.TOOLCHAINDIR}
.DESTDIR				?= /
else

# Cross build
.CONFIGURE_ARGS			?= -DCMAKE_INSTALL_PREFIX=/usr
.EXTRA_CONFIGURE_ARGS	+= -DCMAKE_TOOLCHAIN_FILE=${.OBJDIR}/avyos.${TARGET_TRIPLET}.cmake
.DESTDIR 				?= ${.SYSROOTDIR}

define AVYOS_TOOLCHAIN_FILE
set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR ${TARGET_ARCH})

set(CMAKE_C_COMPILER		${CC})
set(CMAKE_C_COMPILER_TARGET		${TARGET_TRIPLET})

set(CMAKE_CXX_COMPILER			${CXX})
set(CMAKE_CXX_COMPILER_TARGET	${TARGET_TRIPLET})

set(CMAKE_ASM_COMPILER			${AS})
set(CMAKE_ASM_COMPILER_TARGET	${TARGET_TRIPLET})

set(CMAKE_LINKER				${LD})
set(CMAKE_LINKER_TARGET			${TARGET_TRIPLET})


set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)

set(CMAKE_SYSROOT       "${.SYSROOTDIR}")

set(CMAKE_C_FLAGS		"${CFLAGS}")
set(CMAKE_CXX_FLAGS		"${CXXFLAGS}")
set(CMAKE_ASM_FLAGS		"${CFLAGS}")

set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
endef

export AVYOS_TOOLCHAIN_FILE

endif


all: ${.OBJDIR}/.avyos-install-done

clean:
	rm -rf ${.OBJDIR}/.avyos-*-done

compdb:
	true

${.OBJDIR}/.avyos-install-done: ${.OBJDIR}/.avyos-compile-done
	set -e; cd ${.OBJDIR}; ${.PRE_INSTALL_COMMANDS}

	DESTDIR=${.DESTDIR} cmake --install ${.OBJDIR} ${.INSTALL_ARGS}

	set -e; cd ${.OBJDIR}; ${.POST_INSTALL_COMMANDS}

	@touch $@

${.OBJDIR}/.avyos-compile-done: ${.OBJDIR}/.avyos-configure-done
	set -e; cd ${.OBJDIR}; ${.PRE_COMPILE_COMMANDS}

	cmake --build ${.OBJDIR} ${.COMPILE_ARGS}

	set -e; cd ${.OBJDIR}; ${.POST_COMPILE_COMMANDS}

	@touch $@

${.OBJDIR}/.avyos-configure-done: ${.DISTDIR}/CMakeLists.txt
ifneq (${.HOSTBUILD},1)
	echo "$$AVYOS_TOOLCHAIN_FILE" > ${.OBJDIR}/avyos.${TARGET_TRIPLET}.cmake
endif
	set -e; cd ${.OBJDIR}; ${.PRE_CONFIGURE_COMMANDS}
	cmake -B ${.OBJDIR} -S ${.DISTDIR} ${.CONFIGURE_ARGS} ${.EXTRA_CONFIGURE_ARGS}
	set -e; cd ${.OBJDIR}; ${.POST_CONFIGURE_COMMANDS}

	@touch $@

include ${.TOPDIR}/build/avyos.sources.inc
