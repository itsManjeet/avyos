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

# Check that DejaGnu testsuites have 'srcdir' defined to a relative path
# (both as TCL variable and as environment variable).

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
EXTRA_DIST  = env.test/env.exp tcl.test/tcl.exp
EXTRA_DIST += lib/tcl.exp
END

mkdir testsuite/env.test testsuite/tcl.test testsuite/lib

# DejaGnu can change $srcdir behind our backs, so we have to
# save its original value.  Thanks to Ian Lance Taylor for the
# suggestion.
cat > testsuite/lib/tcl.exp << 'END'
send_user "tcl_lib_srcdir: $srcdir\n"
set orig_srcdir $srcdir
END

cat > testsuite/env.test/env.exp << 'END'
set env_srcdir $env(srcdir)
send_user "env_srcdir: $env_srcdir\n"
if { [ regexp {^\.(\./\.\./\.\./testsuite)?$} $env_srcdir ] } {
    pass "test_env_src"
} else {
    fail "test_env_src"
}
END

cat > testsuite/tcl.test/tcl.exp << 'END'
send_user "tcl_srcdir: $srcdir\n"
if { [ regexp {^\.(\./\.\./\.\./testsuite)?$} $srcdir ] } {
    pass "test_tcl_src"
} else {
    fail "test_tcl_src"
}
send_user "tcl_orig_srcdir: $orig_srcdir\n"
if { [ regexp "^\.(\./\.\./\.\./testsuite)?$" $orig_srcdir ] } {
    pass "test_tcl_orig_src"
} else {
    fail "test_tcl_orig_src"
}
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

./configure --srcdir=.

$MAKE check

# Sanity check: all tests have run.
test -f testsuite/env.log
test -f testsuite/env.sum
test -f testsuite/tcl.log
test -f testsuite/tcl.sum

$MAKE distcheck

:
