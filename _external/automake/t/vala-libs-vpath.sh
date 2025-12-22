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
AC_INIT([valalibs],[0.1])

AC_CONFIG_MACRO_DIR([m4])

AM_INIT_AUTOMAKE
AM_PROG_AR
LT_INIT

AC_PROG_CC

AM_PROG_VALAC([0.7.3])
PKG_CHECK_MODULES([GOBJECT], [gobject-2.0 >= 2.4])

AC_CONFIG_FILES([Makefile src/Makefile])
AC_OUTPUT
END


cat > Makefile.am << 'END'
SUBDIRS=src
END

mkdir src

cat > src/Makefile.am << 'END'
AUTOMAKE_OPTIONS = subdir-objects
lib_LTLIBRARIES = libservice.la
libservice_la_SOURCES = service.vala cservice.c cservice.h
libservice_la_CPPFLAGS = -DOKOKIMDEFINED=1
libservice_la_VALAFLAGS = --vapidir=$(srcdir) --pkg cservice --library service
AM_CFLAGS = $(GOBJECT_CFLAGS)
END

libtoolize
$ACLOCAL
$AUTOCONF
$AUTOMAKE -a

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

cat > src/service.vala << 'END'
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

mkdir build
cd build
../configure

$MAKE
pwd
test -f src/libservice_la_vala.stamp
test -f src/libservice.la

:
