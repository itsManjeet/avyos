ifneq (${HOST_BUILD},1)
export CC       				:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-gcc
export CXX      				:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-g++
export AS						:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-gcc
export LD						:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ld
export AR						:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ar
export STRIP					:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-strip
export RANLIB					:= ${TOOLCHAIN_PATH}/bin/${TARGET_TRIPLET}-ranlib

export PKG_CONFIG_SYSROOT_DIR	:= ${SYSTEM_PATH}
export PKG_CONFIG_LIBDIR		:= ${SYSTEM_PATH}/lib/pkgconfig:${SYSTEM_PATH}/data/pkgconfig

ifeq (${GOARCH},amd64)
export CFLAGS					+= -march=x86-64
export CXXFLAGS					+= -march=x86-64
endif

endif
