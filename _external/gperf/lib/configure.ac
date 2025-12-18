dnl autoconf configuration for gperf/lib

dnl Copyright (C) 1998-2025 Free Software Foundation, Inc.
dnl Written by Douglas C. Schmidt <schmidt@ics.uci.edu>
dnl and Bruno Haible <bruno@clisp.org>.
dnl
dnl This file is part of GNU GPERF.
dnl
dnl This program is free software: you can redistribute it and/or modify
dnl it under the terms of the GNU General Public License as published by
dnl the Free Software Foundation; either version 3 of the License, or
dnl (at your option) any later version.
dnl
dnl This program is distributed in the hope that it will be useful,
dnl but WITHOUT ANY WARRANTY; without even the implied warranty of
dnl MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
dnl GNU General Public License for more details.
dnl
dnl You should have received a copy of the GNU General Public License
dnl along with this program.  If not, see <https://www.gnu.org/licenses/>.

AC_PREREQ([2.70])
AC_INIT([gperf], [package])
AC_CONFIG_SRCDIR([hash.cc])
AC_CONFIG_AUX_DIR([../build-aux])
AC_CONFIG_MACRO_DIRS([gnulib-m4])
VERSION_NUMBER=`sed -n -e 's/.*version_string = "\(.*\)".*/\1/p' < $srcdir/../src/version.cc`
gl_INIT_PACKAGE_VERSION([$VERSION_NUMBER])
AM_INIT_AUTOMAKE
AC_CONFIG_HEADERS([config.h])
dnl
dnl           checks for programs
dnl
AC_PROG_CC
                      dnl sets variable CC
AC_PROG_CXX
                      dnl sets variable CXX
gl_EARLY
                      dnl Early macro expansions from gnulib
AC_CHECK_TOOL([AR], [ar])
                      dnl sets variable AR
AC_PROG_RANLIB
                      dnl sets variable RANLIB
gl_INIT
                      dnl Main macro expansions from gnulib
dnl
dnl           That's it.
dnl
AC_CONFIG_FILES([Makefile])
AC_OUTPUT
