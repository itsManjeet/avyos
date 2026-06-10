.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

SOURCES = ${PACKAGE_LLVM_URI}
.DISTDIR = ${SOURCESDIR}/llvm-project-${PACKAGE_LLVM_VERSION}.src/runtimes

.EXTRA_CONFIGURE_ARGS += \
	-D LLVM_ENABLE_RUNTIMES='libcxx;libcxxabi;libunwind' \
	-D LLVM_ENABLE_PIC=ON \
	-D LIBCXX_INSTALL_MODULES=ON \
	-D LIBCXXABI_USE_LLVM_UNWINDER=ON \
	-D LIBCXXABI_HAS_CXA_THREAD_ATEXIT_IMPL=OFF \
	-D LIBCXX_EXTRA_SITE_DEFINES="_LIBCPP_PROVIDES_DEFAULT_RUNE_TABLE"

.COMPILE_ARGS += --target cxx cxxabi unwind
# .INSTALL_ARGS += --target install-cxx install-cxxabi install-unwind

include ${.TOPDIR}/build/avyos.cmake.inc
