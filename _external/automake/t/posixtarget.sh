#! /bin/sh
# Copyright (C) 2023-2025 Free Software Foundation, Inc.
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

# Check that the posix option is supported. See https://bugs.gnu.org/55025.

. test-init.sh

cat > configure.ac << 'END'
AC_INIT([posixtest], [0.0])
AM_INIT_AUTOMAKE([posix foreign])
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
# Some comment.
random-target:
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE

# .POSIX should be the first non-blank non-comment line.
sed -e '/^$/d' -e '/^ *#/d' -e 1q Makefile.in | grep '^\.POSIX:'

./configure
# Although we aren't responsible for what autoconf does, check that the
# result is as expected, since we're here.
sed -e '/^$/d' -e '/^ *#/d' -e 1q Makefile | grep '^\.POSIX:'
