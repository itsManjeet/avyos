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

# If IGNORE_SKIPPED_LOGS is set, skipped tests should not be in test-suite.log.

. test-init.sh

cat >>configure.ac <<END
AC_OUTPUT
END

cat >Makefile.am <<'END'
LOG_COMPILER = $(SHELL)
TESTS = pass fail skiptest xpass xfail error
XFAIL_TESTS = xpass xfail
END

echo 'exit 0' > pass
echo 'exit 0' > xpass
echo 'exit 1' > fail
echo 'exit 1' > xfail
echo 'exit 77' > skiptest # unique name for grep
echo 'exit 99' > error

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

# Check that IGNORE_SKIPPED_LOGS works when given on the command line.

./configure
run_make -e FAIL check IGNORE_SKIPPED_LOGS=true
grep skiptest test-suite.log && exit 1

rm -f test-suite.log

# Check that IGNORE_SKIPPED_LOGS works when given in the Makefile.

cat >>Makefile.am <<'END'
IGNORE_SKIPPED_LOGS = true
END

$AUTOMAKE --add-missing

./configure
run_make -e FAIL check
grep skiptest test-suite.log && exit 1

:
