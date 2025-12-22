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

# Test that 'install-exec:' honors $(BUILT_SOURCES);
# https://bugs.gnu.org/43683.

. test-init.sh

cat >> configure.ac << 'END'
AC_OUTPUT
END

cat > Makefile.am << 'END'
BUILT_SOURCES = built1
built1:
	echo ok > $@
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE
./configure --prefix "$(pwd)/inst"

# Make sure this file is rebuilt by make install-exec.
$MAKE install-exec
test -f built1

:
