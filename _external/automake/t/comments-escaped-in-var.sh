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

# Make sure Automake preserves escaped comments in the output.

. test-init.sh

cat >> configure.ac <<'END'
AC_OUTPUT
END

cat > Makefile.am << 'END'
# This value should be preserved.
var = -DVAL='"\#xxx\#"'   # this should be stripped
var += -DX=1  # this should be kept
END

$ACLOCAL

# This should fail due to -Werror, as in:
#   automake-1.16: warnings are treated as errors
#   Makefile.am:2: warning: escaping \# comment markers is not portable
AUTOMAKE_fails
grep 'escaping.*comment markers.*portable' stderr

# This should pass though.
$AUTOMAKE -Wno-portability

# For debugging.
grep ^var Makefile.in

# The full flag should be retained.
grep '^var.*\\#xxx\\#.*DX=1' Makefile.in

# Only the 2nd comment should be retained.
grep '^var.*stripped' Makefile.in && exit 1 || :
grep '^var.*should be kept' Makefile.in
