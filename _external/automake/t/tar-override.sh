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

# Check that the user can override the tar program used by "make dist"
# at runtime, by redefining the 'TAR' environment variable.
# 
# Currently this works only when the tar format used is 'v7';
# as of 2025 (automake-1.18), this is no longer the default,
# so force that format in our test setup.

. test-init.sh

cwd=$(pwd) || fatal_ "getting current working directory"

cat > configure.ac << 'END'
AC_INIT([tar-override], [1.0])
AM_INIT_AUTOMAKE([tar-v7])
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
END

cat > am--tar <<'END'
#!/bin/sh
echo $1 > am--tar-has-run
shift
exec tar "$@"
END
chmod a+x am--tar

cat > Makefile.am <<'END'
check-local: dist
	ls -l ;: For debugging.
	test -f am--tar-has-run
CLEANFILES = am--tar-has-run
END

$ACLOCAL
$AUTOCONF
$AUTOMAKE
./configure

clean_temp () { rm -f *.tar.* *has-run*; }

$MAKE dist
test -f $distdir.tar.gz
ls | grep has-run && exit 1

clean_temp

TAR="$cwd/am--tar foo" $MAKE distcheck
test -f $distdir.tar.gz
test "$(cat am--tar-has-run)" = foo

clean_temp

unset TAR
# Creative use of eval to pacify maintainer checks.
eval \$'MAKE dist "TAR=./am--tar mu"'
test -f $distdir.tar.gz
test "$(cat am--tar-has-run)" = mu

:
