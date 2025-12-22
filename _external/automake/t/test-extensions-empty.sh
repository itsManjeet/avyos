#! /bin/sh
# Copyright (C) 2020-2025 Free Software Foundation, Inc.
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

# Empty assignment to TEST_EXTENSIONS should not provoke Perl warning.
# https://bugs.gnu.org/42635

. test-init.sh

cat > configure.ac << 'END'
AC_INIT([foo],[1.0])
AM_INIT_AUTOMAKE([foreign])
AC_PROG_CC  dnl comment this line to make the warning disappear
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
TEST_EXTENSIONS =
LOG_COMPILER = echo
TESTS = foo.test
END

touch foo.test

autoreconf -fi >reconf.out 2>&1
grep 'uninitialized value' reconf.out && exit 1

# What we're trying to avoid:
# ...
# Use of uninitialized value in string eq at /usr/bin/automake line 4953.
# ...
# nl -ba `command -v automake` | sed -n '4951,4955p'
#  4951            if ($handle_exeext)
#  4952              {
#  4953                unshift (@test_suffixes, $at_exeext)
#  4954                  unless $test_suffixes[0] eq $at_exeext;
#  4955              }

:
