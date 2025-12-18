#! /bin/sh
# Copyright (C) 2022-2025 Free Software Foundation, Inc.
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

# Make sure auxdir with subdir aux files works.
# https://bugs.gnu.org/20300

. test-init.sh

cat > configure.ac <<END
AC_INIT([$me], [1.0])
AC_CONFIG_AUX_DIR([build-aux])
AC_REQUIRE_AUX_FILE([top-file])
AC_REQUIRE_AUX_FILE([subdir/file])
AM_INIT_AUTOMAKE
AC_CONFIG_FILES(Makefile)
END

touch Makefile.am

mkdir -p build-aux/subdir
: >build-aux/top-file
: >build-aux/subdir/file

$ACLOCAL
$AUTOMAKE --add-missing

:
