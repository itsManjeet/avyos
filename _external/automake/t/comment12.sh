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

# Make sure that earlier comments are consumed when appending to it.

. test-init.sh

cat > Makefile.am << 'END'
VAR1=# eat this comment
VAR2 =# eat this comment
VAR3 = # eat this comment
VAR4 =	# eat this comment
VAR5 = val# eat this comment
VAR6 = val # eat this comment

VAR1 += val
VAR2 += val
VAR3 += val
VAR4 += val
VAR5 +=
VAR6 +=
END

$ACLOCAL
$AUTOMAKE

# For debugging.
grep '^VAR' Makefile.in

count=$(grep '^VAR. = val$' Makefile.in | wc -l)
[ $count -eq 6 ]
