#! /bin/sh
# Copyright (C) 2002-2025 Free Software Foundation, Inc.
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

# Check basic gettext support.

required='gettext'
. test-init.sh

cat >> configure.ac << 'END'
AM_GNU_GETTEXT([external])
AC_OUTPUT
END

: > Makefile.am
: > config.rpath
mkdir po intl

$ACLOCAL
$AUTOCONF

# po/ is required.  intl/ may not be used with external gettext.
# Internal (bundled) was deprecated upstream in gettext 0.18 (2010)
# and made fatal in gettext 0.20 (2019).

AUTOMAKE_fails --add-missing
grep 'AM_GNU_GETTEXT.*SUBDIRS' stderr

echo 'SUBDIRS = po' >Makefile.am
# Should not fail.
$AUTOMAKE --add-missing

echo 'SUBDIRS = intl' >Makefile.am
AUTOMAKE_fails --add-missing
grep 'AM_GNU_GETTEXT.*po' stderr

echo 'SUBDIRS = po intl' >Makefile.am
AUTOMAKE_fails --add-missing
grep 'intl.*SUBDIRS.*AM_GNU_GETTEXT' stderr

:
