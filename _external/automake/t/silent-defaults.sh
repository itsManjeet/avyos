#!/bin/sh
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

# Check verbose mode defaults and behavior. See bug#32868.
# Because we have to rerun the autotools for every configuration,
# this test can take 30 seconds or so to run.

. test-init.sh

: > Makefile.am

# 
echo "Default behavior is currently verbose."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-default-verbose], [1.0])
AM_INIT_AUTOMAKE
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User doesn't pick a silent mode default before AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-use-am_silent_rules], [1.0])
AM_SILENT_RULES
AM_INIT_AUTOMAKE
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User disables silent mode default before AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-user-disable-before-am_init], [1.0])
AM_SILENT_RULES([no])
AM_INIT_AUTOMAKE
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User enables silent mode default before AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-user-enable-before-am_init], [1.0])
AM_SILENT_RULES([yes])
AM_INIT_AUTOMAKE
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User doesn't pick a silent mode default after AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-user-no-default-after-am_init], [1.0])
AM_INIT_AUTOMAKE
AM_SILENT_RULES
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User disables silent mode default after AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-user-disable-after-am_init], [1.0])
AM_INIT_AUTOMAKE
AM_SILENT_RULES([no])
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

# 
echo "User enables silent mode default after AM_INIT_AUTOMAKE."
cat <<EOF >configure.ac
AC_INIT([silent-defaults-user-enable-after-am_init], [1.0])
AM_INIT_AUTOMAKE
AM_SILENT_RULES([yes])
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
EOF

$ACLOCAL
$AUTOMAKE
$AUTOCONF

./configure
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --enable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 0' Makefile

./configure -C --disable-silent-rules
grep '^AM_DEFAULT_VERBOSITY = 1' Makefile

:
