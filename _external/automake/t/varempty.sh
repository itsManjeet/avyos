#! /bin/sh
# Copyright (C) 2024-2025 Free Software Foundation, Inc.
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

# An empty variable name $() should not cause a Perl warning, namely:
#   Use of uninitialized value $var in string eq at
#   .../lib/Automake/Variable.pm line 754, <GEN2> line 3.
# (in scan_variable_expansions)
# 
# This showed up with the NetworkManager and other packages in Fedora:
# https://lists.gnu.org/archive/html/automake/2024-06/msg00085.html
# (The actual purpose of the "$()" is unclear.)

. test-init.sh

cat > Makefile.am << 'END'
x:
	$()
END

$ACLOCAL
$AUTOMAKE

:
