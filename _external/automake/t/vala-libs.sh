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
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

# Building libraries (libtool and static) from Vala sources.
# And use of vapi files to call C code from Vala.

required="valac cc pkg-config libtoolize GNUmake"
. test-init.sh

cat >> configure.ac << 'END'
AC_PROG_CC
AM_PROG_AR
AC_PROG_RANLIB
AC_PROG_LIBTOOL
AM_PROG_VALAC([0.7.3])
PKG_CHECK_MODULES([GOBJECT], [gobject-2.0 >= 2.4])
AC_OUTPUT
END

cat > Makefile.am << 'END'
AUTOMAKE_OPTIONS = subdir-objects
lib_LIBRARIES = libservice.a
lib_LTLIBRARIES = src/libzardoz.la
libservice_a_SOURCES = service.vala cservice.c cservice.h
libservice_a_CPPFLAGS = -DOKOKIMDEFINED=1
libservice_a_VALAFLAGS = --vapidir=$(srcdir) --pkg cservice --library service
AM_CFLAGS = $(GOBJECT_CFLAGS)
src_libzardoz_la_LIBADD = $(GOBJECT_LIBS)
src_libzardoz_la_SOURCES = src/zardoz-foo.vala src/zardoz-bar.vala
src/zardoz-bar.vala: src/zardoz-foo.vala
	sed 's/Foo/Bar/g' $< >$@
END

libtoolize
$ACLOCAL
$AUTOCONF
$AUTOMAKE -a

./configure

cat > cservice.c << 'END'
#include "cservice.h"
int c_service_mu_call (void)
{
  return OKOKIMDEFINED;
}
END

cat > cservice.h << 'END'
int c_service_mu (void);
END

cat > cservice.vapi <<'END'
namespace CService {
  public class Mu {
    [CCode (cheader_filename = "cservice.h", cname = "c_service_mu_call")]
    public int call ();
  }
}
END

cat > service.vala << 'END'
namespace CService {
public class Generator : Object {
	public Generator () {
		stdout.printf ("construct generator");
	}
	public void init () {
		stdout.printf ("init generator");
	}
}
}
END

mkdir -p src
cat > src/zardoz-foo.vala << 'END'
using GLib;
public class Foo {
  public static void zap () {
    stdout.printf ("FooFooFoo!\n");
  }
}
END

$MAKE
test -f libservice.a
test -f src/libzardoz.la
$FGREP "construct generator" service.c
$FGREP "FooFooFoo" src/zardoz-foo.c
$FGREP "BarBarBar" src/zardoz-bar.c
test -f libservice_a_vala.stamp
test -f src_libzardoz_la_vala.stamp

$MAKE distcheck

:
