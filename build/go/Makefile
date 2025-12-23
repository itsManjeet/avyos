.TOPDIR ?= ../..
include ${.TOPDIR}/build/avyos.defaults.inc

DISTDIR := ${.TOPDIR}/_external/go

export GOROOT_FINAL := ${TOOLCHAIN_PATH}/lib/go

all: ${.OBJDIR}/.install-done


${.OBJDIR}/.install-done: ${.OBJDIR}/.build-done
	@mkdir -p ${TOOLCHAIN_PATH}/bin
	@mkdir -p ${TOOLCHAIN_PATH}/lib/go/
	cd ${DISTDIR}; \
		cp -a bin pkg src lib misc api test \
			${TOOLCHAIN_PATH}/lib/go/;
	ln -sf ../lib/go/bin/go ${TOOLCHAIN_PATH}/bin/go
	ln -sf ../lib/go/bin/gofmt ${TOOLCHAIN_PATH}/bin/gofmt 

	rm -rf ${TOOLCHAIN_PATH}/lib/go/pkg/bootstrap
	rm -rf ${TOOLCHAIN_PATH}/lib/go/pkg/obj/go-build

	install -Dm 0644 ${DISTDIR}/go.env ${TOOLCHAIN_PATH}/lib/go/go.env
	@touch $@


${.OBJDIR}/.build-done: ${DISTDIR}/src/make.bash
	cd $(dir $<) && $<
	@touch $@

clean:
	rm -rf ${.OBJDIR}/.*-done