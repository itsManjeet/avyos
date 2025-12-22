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

# Check that the top-level files (INSTALL, NEWS, README-alpha, etc.)
# can be .md, or not. (Based on alpha2.sh.)

. test-init.sh

cat > configure.ac << 'END'
AC_INIT([alpha], [1.0b])
AM_INIT_AUTOMAKE([readme-alpha])
AC_CONFIG_FILES([Makefile sub/Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
SUBDIRS = sub
check-local: distdir
	for f in AUTHORS ChangeLog INSTALL NEWS README THANKS; do \
	  test -f $(distdir)/$$f.md; done
	test -f $(distdir)/COPYING
	test -f $(distdir)/README-alpha.md
	test ! -f $(distdir)/sub/README.md
	test ! -f $(distdir)/sub/README-alpha.md # not distributed
	: > works
END

mkdir sub
: > sub/Makefile.am

# do both md and non-md.
: > README-alpha.md
: > sub/README-alpha.md
: > sub/README

# top level
: > AUTHORS.md
: > ChangeLog.md
: > INSTALL.md
: > NEWS.md
: > README.md
: > THANKS.md

# not md
: > COPYING


$ACLOCAL
$AUTOCONF
$AUTOMAKE
./configure
$MAKE check
test -f works

