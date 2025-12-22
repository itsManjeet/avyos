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

# Check that we can override the 'dvi' target run as part of distcheck,
# specifically to be 'html', so that TeX is not required.
# Related to automake bug#8289.

# TeX and texi2dvi should not be needed or invoked.
TEX=false TEXI2DVI=false
export TEX TEXI2DVI

required='makeinfo'
. test-init.sh

cat >> configure.ac << 'END'
AC_OUTPUT
END

cat > Makefile.am << 'END'
AM_DISTCHECK_DVI_TARGET = html
info_TEXINFOS = main.texi
END

# Protect with leading " # " to avoid spurious maintainer-check failures.
sed 's/^ *# *//' > main.texi << 'END'
 # \input texinfo
 # @setfilename main.info
 # @settitle main
 #
 # @node Top
 # Hello.
 # @bye
END

$ACLOCAL
$AUTOMAKE -a
$AUTOCONF

./configure
$MAKE
$MAKE distcheck

:
