#! /bin/sh
# Copyright (C) 2011-2025 Free Software Foundation, Inc.
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

# Check that DejaGnu testsuites have 'objdir' defined (as a TCL variable)
# to an absolute path.

required=runtest
. test-init.sh

cat >> configure.ac << 'END'
AC_CONFIG_FILES([testsuite/Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
SUBDIRS = testsuite
END

mkdir testsuite

cat > testsuite/Makefile.am << 'END'
AUTOMAKE_OPTIONS = dejagnu
DEJATOOL = tcl env
EXTRA_DIST = tcl.test/tcl.exp
END

mkdir testsuite/tcl.test

cat > testsuite/tcl.test/tcl.exp << 'END'
send_user "tcl_objdir: $objdir\n"
if { [ regexp "^/" $objdir ] } {
    pass "test_tcl_objdir"
} else {
    fail "test_tcl_objdir"
}
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

./configure

$MAKE check

# Sanity check: all tests have run.
test -f testsuite/env.log
test -f testsuite/env.sum
test -f testsuite/tcl.log
test -f testsuite/tcl.sum

$MAKE distcheck

:
