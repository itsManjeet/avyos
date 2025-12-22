#! /bin/sh
# Copyright (C) 2025 Free Software Foundation, Inc.
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

# Basic check for Algol 68 support.

required=ga68
am_create_testdir=empty
. test-init.sh

cat > configure.ac << 'END'
AC_INIT([GNU Algol 68 Demo], [1.0], [bug-automake@gnu.org])
AC_CONFIG_SRCDIR([play.a68])
AC_CONFIG_AUX_DIR([build-aux])
AM_INIT_AUTOMAKE
AC_PROG_A68
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
AUTOMAKE_OPTIONS = subdir-objects

bin_PROGRAMS = work play
play_SOURCES = play.a68
work_SOURCES = work.a68

.PHONY: test-objs
check-local: test-objs
test-objs:
	test -f play.$(OBJEXT)
	test -f work.$(OBJEXT)
END

mkdir sub build-aux

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

cat > work.a68 << 'END'
(puts ("We are working :-('n"))
END

cat > play.a68 << 'END'
(puts ("We are playing :-)'n"))
END

./configure
$MAKE
$MAKE test-objs

if ! cross_compiling; then
  unindent > exp.play << 'END'
    We are playing :-)
END
  unindent > exp.work << 'END'
    We are working :-(
END
  for p in play work; do
    # The program must run correctly (exit status = 0).
    ./$p
    # And it must have the expected output.  Note that we strip extra
    # CR characters (if any), to cater to MinGW programs on MSYS.
    # See automake bug#14493.
    ./$p | tr -d '\015' > got.$p || { cat got.$p; exit 1; }
    cat exp.$p
    cat got.$p
    diff exp.$p got.$p
  done
fi

$MAKE distcheck

:
