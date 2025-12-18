#! /bin/sh
# Copyright (C) 2019-2025 Free Software Foundation, Inc.
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

# Test to ensure the space after $(LISP) for make tags is present
# if there are CONFIG_HEADERS.
# See automake bug#38139.

required=etags
. test-init.sh

# some AC_CONFIG_FILES header is needed to trigger the bug.
cat >> configure.ac <<'END'
AC_CONFIG_HEADERS([config.h])
AM_PATH_LISPDIR
AC_OUTPUT
END

cat > Makefile.am <<'END'
lisp_LISP = the-amtest-mode.el
END

touch config.h.in
touch the-amtest-mode.el

$ACLOCAL
$AUTOCONF
$AUTOMAKE

./configure
run_make -O -E tags

# make tags should fail if the problem exists, but just in case, match:
# make: *** No rule to make target 'the-amtest-mode.elconfig.h.in', needed by 'tags-am'.  Stop.
grep 'No rule to make target' stderr && exit 1

:
