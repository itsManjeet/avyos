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

# Test for a behavior of 'TESTS_ENVIRONMENT' and 'AM_TESTS_ENVIRONMENT'
# w.r.t. file descriptor redirections which, although undocumented,
# is nonetheless required by Gnulib's 'tests/init.sh' and by coreutils'
# testsuite.
# The checked behavior is that we can portably do file descriptor
# redirections by placing them at the end of a {AM_,}TESTS_ENVIRONMENT
# definition without a following semicolon.  The need to support this
# is motivated in detail by coreutils bug#8846:
#   <https://debbugs.gnu.org/cgi/bugreport.cgi?bug=8846>
# and the following CC:ed thread on bug-autoconf list:
#   <https://lists.gnu.org/archive/html/bug-autoconf/2011-06/msg00002.html>

. test-init.sh

cat >> configure.ac << 'END'
AC_OUTPUT
END

# Use both a shell script and a perl script as tests,
# for better coverage.

cat >foo.test <<'END'
#! /bin/sh
set -e
echo " " $0: foofoofoo >&8
echo " " $0: barbarbar >&9
END

echo "#! $PERL -w" > bar.test
cat >>bar.test <<'END'
use strict;
use warnings;
open(FD8, ">&=8") or die "$!";
open(FD9, ">&=9") or die "$!";
print FD8 "  $0: 8888\n";
print FD9 "  $0: 9999\n";
END

chmod a+x foo.test bar.test

$ACLOCAL
$AUTOCONF

# Korn Shells seem more vulnerable to the issue highlighted in coreutils
# bug#8846 than other shells are.  In particular, the default Korn Shell
# on Debian GNU/Linux is affected by the issue.  So let's try to run our
# test with a system Korn Shell too, if that's available.
bin_ksh=:
case $SHELL in
  ksh|*/ksh) ;;
  *) for d in /bin /usr/bin; do
       test -f $d/ksh && { bin_ksh=$d/ksh; break; }
     done;;
esac

for sh in "$SHELL" "$bin_ksh"; do

  # The following hangs with Fedora 34's ksh-20120801-255:
  # (seq --format=z%g= 121; echo "eval ': \`(set) 2>&1\`'")|env -i ksh
  # yet terminates if it emits one fewer variable assignment.
  # Using that ksh here would make this test hang: the above is
  # derived from the configure script generated for this test.
  # Reported as https://github.com/ksh93/ksh/issues/316
  case $sh in */ksh) skip_ "skipping $sh to avoid hang"; continue;; esac

  test "$sh" = : && continue
  for pfx in AM_ ''; do
    unindent > Makefile.am <<END
      TESTS = foo.test bar.test
      ## No trailing semicolon here, *deliberately*.
      ${pfx}TESTS_ENVIRONMENT = 8>&1 9>&8
END
    $AUTOMAKE -a
    CONFIG_SHELL="$sh" $sh ./configure CONFIG_SHELL="$sh"
    run_make -O VERBOSE=y check
    grep '[ /]foo\.test: foofoofoo$' stdout
    grep '[ /]foo\.test: barbarbar$' stdout
    grep '[ /]bar\.test: 8888$' stdout
    grep '[ /]bar\.test: 9999$' stdout
    $EGREP '(foofoofoo|barbarbar|8888|9999)' foo.log && exit 1
    : # For shells with buggy 'set -e'.
  done
done

:
