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

# Check that the user can edit the Makefile-generated site.exp, and
# have its edits survive to the remaking of that file.

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
DEJATOOL = foo
END

# Deliberately select a variable defined automatically by
# the Makefile-generated site.exp.
mkdir testsuite/foo.test
cat > testsuite/foo.test/foo.exp << 'END'
send_user "objdir: $objdir\n"
set pipe "|"
if { $objdir == "${pipe}objdir${pipe}" } {
    pass "test_obj"
} else {
    fail "test_obj"
}
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

./configure

(cd testsuite/ && $MAKE site.exp)
echo 'set objdir "|objdir|"' >> testsuite/site.exp
cat testsuite/site.exp
$sleep
touch testsuite/Makefile
(cd testsuite/ && $MAKE site.exp)
cat testsuite/site.exp
is_newest testsuite/site.exp testsuite/Makefile  # Sanity check.
grep '|objdir|' testsuite/site.exp
test $($FGREP -c '|objdir|' testsuite/site.exp) -eq 1

# We can do a "more semantic" check if DejaGnu is available.
if runtest SOMEPROGRAM=someprogram --version; then
  $MAKE check
  grep 'PASS: test_obj' testsuite/foo.sum
fi

:
