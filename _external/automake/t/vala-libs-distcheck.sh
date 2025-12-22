#! /bin/sh
# Copyright (C) 2012-2025 Free Software Foundation, Inc.
#
# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2, or (at your option)
# any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

# Building libraries (libtool and static) from Vala sources.
# And use of vapi files to call C code from Vala.

required="valac cc pkg-config libtoolize GNUmake"
am_create_testdir=empty
. test-init.sh

cat >> configure.ac << 'END'
AC_INIT([atest],[0.1])
AC_CONFIG_SRCDIR([data/atest.pc.in])
AC_SUBST([API_VERSION],[0])

AM_INIT_AUTOMAKE
AM_MAINTAINER_MODE([enable])
AM_PROG_AR
LT_INIT

AC_PROG_CC
AC_PROG_INSTALL
PKG_PROG_PKG_CONFIG([0.22])
AM_PROG_VALAC([0.32])

PKG_CHECK_MODULES(ATEST, [gio-2.0])

AC_CONFIG_FILES([
	Makefile

	src/Makefile

	src/atest-$API_VERSION.deps:src/atest.deps.in

	data/Makefile
	data/atest-$API_VERSION.pc:data/atest.pc.in

],[],
[API_VERSION='$API_VERSION'])
AC_OUTPUT
END


cat > Makefile.am << 'END'
SUBDIRS=data src
END

mkdir data

cat > data/atest.pc.in << 'END'
prefix=@prefix@
exec_prefix=@exec_prefix@
libdir=@libdir@
datarootdir=@datarootdir@
datadir=@datadir@
includedir=@includedir@

Name: atest-@API_VERSION@
Description: atest library
Version: @VERSION@
Requires: glib-2.0 gobject-2.0
Libs: -L${libdir} -latest-@API_VERSION@
Cflags: -I${includedir}/atest-@API_VERSION@
END


cat > data/Makefile.am << 'END'
# pkg-config data
# Note that the template file is called atest.pc.in, but generates a
# versioned .pc file using some magic in AC_CONFIG_FILES.
pkgconfigdir = $(libdir)/pkgconfig
pkgconfig_DATA = atest-$(API_VERSION).pc

DISTCLEANFILES = $(pkgconfig_DATA)
EXTRA_DIST = atest.pc.in
END

mkdir src

cat > src/atest.deps.in << 'END'
glib-2.0
END


cat > src/atest.vala << 'END'
using GLib;

namespace Atest {
	public class A {
		public bool foo() { return false; }
	}
}
END

cat > src/Makefile.am << 'END'
lib_LTLIBRARIES = libatest-@API_VERSION@.la

libatest_@API_VERSION@_la_SOURCES = \
	atest.vala \
	cservice.c \
	cservice.h \
	$(NULL)


libatest_@API_VERSION@_la_CPPFLAGS = \
	-DOKOKIMDEFINED=1 \
	$(NULL)

libatest_@API_VERSION@_la_CFLAGS = \
	$(ATEST_CFLAGS) \
	$(WARN_CFLAGS) \
	$(NULL)

libatest_@API_VERSION@_la_LIBADD = \
	$(ATEST_LIBS) \
	$(NULL)

libatest_@API_VERSION@_la_LDFLAGS = \
	$(WARN_LDFLAGS) \
	$(NULL)

libatest_@API_VERSION@_la_VALAFLAGS = \
	--vapidir=$(VAPIDIR) \
	--vapidir=$(srcdir) \
	--pkg cservice \
	--thread \
	--target-glib=2.44 \
	--pkg glib-2.0 \
	-H atest.h \
	--library atest-@API_VERSION@ \
	$(NULL)

header_DATA=atest.h
headerdir=$(includedir)/atest-@API_VERSION@/atest

atest-@API_VERSION@.deps:
	cp atest.deps atest-@API_VERSION@.deps

vapi_DATA=atest-@API_VERSION@.vapi atest-@API_VERSION@.deps
vapidir=$(VAPIDIR)

CLEANFILES = atest-@API_VERSION@.deps
END


cat > src/cservice.c << 'END'
#include "cservice.h"
int c_service_mu_call (void)
{
  return OKOKIMDEFINED;
}
END

cat > src/cservice.h << 'END'
int c_service_mu (void);
END

cat > src/cservice.vapi <<'END'
namespace CService {
  public class Mu {
    [CCode (cheader_filename = "cservice.h", cname = "c_service_mu_call")]
    public int call ();
  }
}
END

libtoolize
$ACLOCAL
$AUTOCONF
$AUTOMAKE -a

./configure

$MAKE
test -f src/libatest_0_la_vala.stamp
test -f src/libatest-0.la

$MAKE distcheck

:
