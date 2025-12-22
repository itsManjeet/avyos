#! /bin/sh
# Copyright (C) 2009-2025 Free Software Foundation, Inc.
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

# Make sure the documentation targets work as required with BSD make,
# even in the presence of subdirs (requires presence of default *-am rules).

required='makeinfo tex texi2dvi dvips'
. test-init.sh

mkdir sub
cat >>configure.ac <<'END'
AC_CONFIG_FILES([sub/Makefile])
AC_OUTPUT
END
cat >Makefile.am <<'END'
SUBDIRS = sub
END
: >sub/Makefile.am

$ACLOCAL
$AUTOCONF
$AUTOMAKE
./configure --prefix="$(pwd)/inst"
$MAKE html dvi ps pdf info \
      install-html install-dvi install-ps install-pdf install-info \
      install-man install-data install-exec install uninstall

:
