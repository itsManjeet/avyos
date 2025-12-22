#! /bin/sh
# Copyright (C) 2009-2025 Free Software Foundation, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

# Test to make sure _AR variables are detected and used as documented.

. test-init.sh

cat >> configure.ac << 'END'
AC_PROG_CC
AM_PROG_AR
AC_PROG_RANLIB
AC_OUTPUT
END

cat > Makefile.am << 'END'
EXTRA_LIBRARIES = libfoo.a libbar.a
libfoo_a_SOURCES = foo.c
libfoo_a_AR = $(AR)
libbar_a_SOURCES = bar.c
END

$ACLOCAL
$AUTOMAKE -a

# We should use libfoo_a_AR not AR ARFLAGS.
grep '.\$(libfoo_a_AR) *libfoo.a' Makefile.in
grep '.\$(AR).*libfoo.a' Makefile.in && exit 1
grep '.\$(ARFLAGS).*libfoo.a' Makefile.in && exit 1
grep '^ *libfoo_a_AR *=.*\$(ARFLAGS)' Makefile.in && exit 1

# We should use default AR ARFLAGS.
grep '^ *libbar_a_AR *=.*\$(ARFLAGS)' Makefile.in

# Silent make rules should use AM_V_AR.
grep '.\$(AM_V_AR)\$(libbar_a_AR)' Makefile.in

exit 0
