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
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

# Interaction of 'nobase_' and 'dist_' prefixes.

. test-init.sh

cat >> configure.ac <<'EOF'
AC_OUTPUT
EOF

cat > Makefile.am <<'EOF'
foodir = $(prefix)/foo

bardir = $(prefix)/bar

sub/nodist.dat:
	$(MKDIR_P) sub
	: > $@

nobase_foo_DATA = sub/nodist.dat
nobase_dist_foo_DATA = sub/dist.dat

dist_bar_DATA = sub/base.dat
nobase_dist_bar_DATA = sub/nobase.dat

CLEANFILES = sub/nodist.dat
EOF

mkdir sub

: > sub/dist.dat
: > sub/nobase.dat
: > sub/base.dat

rm -f install-sh

$ACLOCAL
$AUTOCONF
$AUTOMAKE -a
./configure --prefix "$(pwd)/inst"

$MAKE
$MAKE install

test -f inst/foo/sub/nodist.dat
test -f inst/foo/sub/dist.dat

test -f inst/bar/sub/nobase.dat
test -f inst/bar/base.dat

$MAKE distcheck

:
