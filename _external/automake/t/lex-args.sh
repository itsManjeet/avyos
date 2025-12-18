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

# Autoconf 2.70 requires AC_PROG_LEX to be called with either 'yywrap'
# or 'noyywrap' as the parameter (previously, the macro had no parameters).
# After updating AM_PROG_LEX, check that either the required parameter values
# are passed down to AC_PROG_LEX, the defaults are used, or a warning is
# issued (and the default is used).
# (parts copied from t/lex-clean.sh)

required='cc lex'
. test-init.sh

expected_errmsg='AC_PROG_LEX without either yywrap or noyywrap'

cp configure.ac configure.bak

cat > Makefile.am << 'END'
bin_PROGRAMS = foo
foo_SOURCES = main.c lexer.l
LDADD = $(LEXLIB)
END

cat > lexer.l << 'END'
%{
#define YY_NO_UNISTD_H 1
%}
%%
"GOOD"   return EOF;
.
END

cat > main.c << 'END'
int main (void) { return yylex (); }
int yywrap (void) { return 1; }
END

for failing in '' '([])' '()';
do
	echo "============== Testing AM_PROG_LEX with >$failing<"

	cat configure.bak - > configure.ac <<END
AC_PROG_CC
AM_PROG_LEX$failing
AC_OUTPUT
END
	# debug:
	#cat configure.ac

	# aclocal seems required every time (at least, if 'make' would be run)
	$ACLOCAL
	# we expect the message, so missing is an error:
	($AUTOCONF 2>&1 | grep "$expected_errmsg") \
		|| (cat configure.ac && exit 1)
	rm -rf autom4te*.cache
done;

for working in '([noyywrap])' '([yywrap])';
do
	echo "============== Testing AM_PROG_LEX with >$working<"

	cat configure.bak - > configure.ac <<END
AC_PROG_CC
AM_PROG_LEX$working
AC_OUTPUT
END
	# debug:
	#cat configure.ac

	$ACLOCAL
	# we don't expect the message, so it is an error if found:
	($AUTOCONF 2>&1 | grep "$expected_errmsg") \
		&& cat configure.ac && exit 2
	rm -rf autom4te*.cache
done;

:
