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

# Check that we can override the "Testsuite summary" header line,
# per bug#11745.

. test-lib.sh

use_colors=no; use_vpath=no
. testsuite-summary-checks.sh

./configure

# Cut down from do_check in ax/testsuite-summary-checks.sh
# so that we can pass a make variable setting in $1.
#
do_header_check ()
{
  cat > summary.exp
  run_make -O -e IGNORE check "$1"
  test $am_make_rc -eq 0 || exit 1
  $PERL "$am_testaux_srcdir"/extract-testsuite-summary.pl stdout >summary.got \
   || fatal_ "cannot extract testsuite summary"
  cat summary.exp
  cat summary.got
  compare=diff
  $compare summary.exp summary.got || exit 1
}

# We don't actually run any tests, only interested in the header line.
results="\
# TOTAL: 0
# PASS:  0
# SKIP:  0
# XFAIL: 0
# FAIL:  0
# XPASS: 0
# ERROR: 0"
#
success_footer=${br}

# Check the default.
header="\
${br}
Testsuite summary for GNU AutoFoo 7.1
${br}"
#
do_header_check 'junkvar=junkval' <<END
$header
$results
$success_footer
END

# Elide the "for $(PACKAGE_STRING)".
header_min="\
${br}
Testsuite summary
${br}"
#
do_header_check 'AM_TESTSUITE_SUMMARY_HEADER=""' <<END
$header_min
$results
$success_footer
END

# Add a suffix.
header_more="\
${br}
Testsuite summary for GNU AutoFoo 7.1 (hi)
${br}"
#
do_header_check 'AM_TESTSUITE_SUMMARY_HEADER=" for $(PACKAGE_STRING) (hi)"' <<END
$header_more
$results
$success_footer
END

:
