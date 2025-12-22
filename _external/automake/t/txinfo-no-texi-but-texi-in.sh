#! /bin/sh
# Copyright (C) 2023-2025 Free Software Foundation, Inc.
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

# Check if Automake doesn't exit with an error for Texinfo output files
# without a direct input file, but with a matching input file processed
# by 'configure' (file.texi.in).
# https://debbugs.gnu.org/cgi/bugreport.cgi?bug=54063#41
#
# This also tests an @setfilename that is different from the file name,
# and the lack of any @setfilename, in the case of using .texi.in.
# (See scan_texinfo_file in automake.)

. test-init.sh

echo AC_OUTPUT >> configure.ac

cat > Makefile.am << 'END'
info_TEXINFOS = main.texi
END

cat > main.texi.in << 'END'
\input texinfo
@setfilename main.info
@settitle main
@node Top
Hello world.
@bye
END

$ACLOCAL
$AUTOMAKE --add-missing

grep '/main.info:' Makefile.in

# Recreate the test document without @setfilename.
cat > main.texi.in << 'END'
\input texinfo
@settitle main
@node Top
Hello world.
@bye
END

$ACLOCAL
$AUTOMAKE

# The result should use main.info (from the .texi file name).
grep '/main.info:' Makefile.in

:
