.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/mesa

EXTRA_CONFIGURE_ARGS += \
	-D platforms=wayland \
	-D glx=disabled \
	-D gallium-drivers=virgl,svga,softpipe \
	-D vulkan-drivers="" \
	-D llvm=disabled

export CFLAGS	+= -DBLAKE3_NO_SSE2 -DBLAKE3_NO_SSE41 -DBLAKE3_NO_AVX2 -DBLAKE3_NO_AVX512
export CXXFLAGS	+= -DBLAKE3_NO_SSE2 -DBLAKE3_NO_SSE41 -DBLAKE3_NO_AVX2 -DBLAKE3_NO_AVX512
 
include ${.TOPDIR}/build/avyos.meson.inc