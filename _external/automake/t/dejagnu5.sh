#! /bin/sh
# Copyright (C) 2003-2025 Free Software Foundation, Inc.
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

# Check that the DejaGnu rules do look for a testsuite named after
# the package by default.

required=runtest
. test-init.sh

package=$me

cat > $package << 'END'
#! /bin/sh
echo "Ah, we have been expecting you, Mr. Blond."
END
chmod +x $package

cat >> configure.ac << 'END'
AC_CONFIG_FILES([testsuite/Makefile])
AC_OUTPUT
END

cat > Makefile.am << END
SUBDIRS = testsuite
EXTRA_DIST = $package
END

mkdir testsuite

cat > testsuite/Makefile.am << END
AUTOMAKE_OPTIONS = dejagnu
EXTRA_DIST = $package.test/$package.exp
AM_RUNTESTFLAGS = PACKAGE=\$(top_srcdir)/$package
END

mkdir testsuite/$package.test
cat > testsuite/$package.test/$package.exp << 'END'
set test "a_dejagnu_test"
spawn $PACKAGE
expect {
    "Ah, we have been expecting you, Mr. Blond." { pass "$test" }
    default { fail "$test" }
}
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

./configure

$MAKE check
test -f testsuite/$package.log
test -f testsuite/$package.sum

$MAKE distcheck

:
