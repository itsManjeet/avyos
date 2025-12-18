#! /bin/sh
# Copyright (C) 1998-2025 Free Software Foundation, Inc.
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

# Test to make sure that pure Fortran 77 sources don't include
# C-specific code.
# Matthew D. Langston <langston@SLAC.Stanford.EDU>

. test-init.sh

cat >> configure.ac << 'END'
AC_PROG_F77
END

cat > Makefile.am << 'END'
sbin_PROGRAMS = anonymous
anonymous_SOURCES = doe.f jane.f
END

: > doe.f
: > jane.f

$ACLOCAL
$AUTOMAKE

# In bug#64743, explicit rules were added to disable make's default
# built-in rules, including one for SCCS:
# %:: SCCS/s.%
# So don't match that. Meanwhile, MKDEP does not occur in the output
# any more, but leaving it in in case it comes back.
grep '[^F]CC' Makefile.in | grep -v MKDEP | grep -v SCCS && exit 1
exit 0
