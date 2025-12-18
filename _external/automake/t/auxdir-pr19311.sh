#! /bin/sh
# Copyright (C) 2014-2025 Free Software Foundation, Inc.
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

# Automake bug#19311: AC_PROG_CC called before AC_CONFIG_AUX_DIR can
# silently force wrong $ac_aux_dir definition.

am_create_testdir=empty
required=cc
. test-init.sh

ver=$($AUTOCONF --version | sed -n '1s/.* //p')
case $ver in
  2.69[d-z]*) ;;
  2.[7-9][0-9]*) ;;
  [3-9].*) ;;
  *) skip_ 'this test passes with autoconf-2.69d and newer'
esac

cat > configure.ac <<END
AC_INIT([$me], [1.0])
AC_PROG_CC
AC_CONFIG_AUX_DIR([build-aux])
AM_INIT_AUTOMAKE
AC_OUTPUT([Makefile])
END

: > Makefile.am

mkdir build-aux

$ACLOCAL
$AUTOMAKE -a
$AUTOCONF

test -f build-aux/compile
test -f build-aux/install-sh

./configure

:
