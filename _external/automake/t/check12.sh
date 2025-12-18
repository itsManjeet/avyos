#! /bin/sh
# Copyright (C) 2010-2025 Free Software Foundation, Inc.
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

# Test to make sure dejagnu tests, automake-style tests, and check-local
# target can coexist.

# For gen-testsuite-part: ==> try-with-serial-tests <==
required=runtest
. test-init.sh

cat >> configure.ac << 'END'
AC_CONFIG_FILES([testsuite/Makefile])
AC_OUTPUT
END

cat > Makefile.am << 'END'
AUTOMAKE_OPTIONS =
EXTRA_DIST =
CLEANFILES =
END

## Simple tests.

cat > a.test << 'END'
#!/bin/sh
echo a.test: exit status: ${A_EXIT_STATUS-0}
exit ${A_EXIT_STATUS-0}
END
cat > b.test << 'END'
#!/bin/sh
echo b.test: exit status: ${B_EXIT_STATUS-0}
exit ${B_EXIT_STATUS-0}
END
chmod +x a.test b.test

cat >> Makefile.am << 'END'
TESTS = a.test b.test
EXTRA_DIST += $(TESTS)
END

A_EXIT_STATUS=0; export A_EXIT_STATUS
B_EXIT_STATUS=0; export B_EXIT_STATUS

## DejaGnu tests.

cat >> Makefile.am << 'END'
SUBDIRS = testsuite
EXTRA_DIST += hammer spanner
END

mkdir testsuite

cat >> testsuite/Makefile.am << 'END'
AUTOMAKE_OPTIONS = dejagnu
DEJATOOL = hammer spanner
AM_RUNTESTFLAGS = HAMMER=$(top_srcdir)/hammer SPANNER=$(top_srcdir)/spanner
EXTRA_DIST = hammer.test/hammer.exp spanner.test/spanner.exp
END

cat > hammer << 'END'
#! /bin/sh
echo "Everything looks like a ${NAIL-nail} to me!"
END

NAIL=nail; export NAIL

cat > spanner << 'END'
#! /bin/sh
echo "I'm a right spanner!"
END
chmod +x hammer spanner

mkdir testsuite/hammer.test testsuite/spanner.test

cat > testsuite/hammer.test/hammer.exp << 'END'
set test test_hammer
spawn $HAMMER
expect {
    "Everything looks like a nail to me!" { pass "$test" }
    default { fail "$test" }
}
END

cat > testsuite/spanner.test/spanner.exp << 'END'
set test test_spanner
spawn $SPANNER
expect {
    "I'm a right spanner!" { pass "$test" }
    default { fail "$test" }
}
END

## User-defined extra tests.

cat >> Makefile.am <<'END'
check-local:
	case $$CHECKLOCAL_EXIT_STATUS in \
	  0) echo 'check-local succeeded :-)';; \
	  *) echo 'check-local failed :-(';; \
	esac >local.log
	exit $$CHECKLOCAL_EXIT_STATUS
CLEANFILES += local.log
END
CHECKLOCAL_EXIT_STATUS=0; export CHECKLOCAL_EXIT_STATUS

## Go with the testcase execution.

$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

for vpath in : false; do

  if $vpath; then
    srcdir=..
    mkdir build
    cd build
  else
    srcdir=.
  fi

  if test -f config.status; then
    $MAKE distclean
  fi

  $srcdir/configure

  $MAKE check
  test -f testsuite/hammer.log
  test -f testsuite/hammer.sum
  test -f testsuite/spanner.log
  test -f testsuite/spanner.sum
  if test x"$am_serial_tests" != x"yes"; then
    test -f test-suite.log
    test -f a.log
    test -f b.log
  fi
  grep 'check-local succeeded :-)' local.log

  cp -f config.status config-status.sav

  $MAKE distclean
  test ! -e testsuite/hammer.log
  test ! -e testsuite/hammer.sum
  test ! -e testsuite/spanner.log
  test ! -e testsuite/spanner.sum
  test ! -e test-suite.log
  test ! -e a.log
  test ! -e b.log
  test ! -e local.log

  mv -f config-status.sav config.status
  ./config.status

  NAIL=screw $MAKE check && exit 1
  test -f testsuite/hammer.log
  test -f testsuite/hammer.sum
  test -f testsuite/spanner.log
  test -f testsuite/spanner.sum
  grep 'FAIL: test_hammer' testsuite/hammer.sum
  grep 'FAIL:' testsuite/spanner.sum && exit 1

  B_EXIT_STATUS=1 $MAKE check && exit 1
  if test x"$am_serial_tests" != x"yes"; then
    cat test-suite.log
    cat a.log
    cat b.log
    grep '^a\.test: exit status: 0$' a.log
    grep '^b\.test: exit status: 1$' b.log
    grep '^FAIL: b$' test-suite.log
    grep '^b\.test: exit status: 1$' test-suite.log
    grep '^a\.test' test-suite.log && exit 1
    : For shells with busted 'set -e'.
  fi

  CHECKLOCAL_EXIT_STATUS=1 $MAKE check && exit 1
  grep 'check-local failed :-(' local.log

  # Do not trust the exit status of 'make -k'.
  NAIL=screw B_EXIT_STATUS=23 CHECKLOCAL_EXIT_STATUS=1 $MAKE -k check || :
  test -f testsuite/hammer.log
  test -f testsuite/hammer.sum
  test -f testsuite/spanner.log
  test -f testsuite/spanner.sum
  grep 'FAIL: test_hammer' testsuite/hammer.sum
  grep 'FAIL:' testsuite/spanner.sum && exit 1
  if test x"$am_serial_tests" != x"yes"; then
    cat test-suite.log
    cat a.log
    cat b.log
    grep '^a\.test: exit status: 0$' a.log
    grep '^b\.test: exit status: 23$' b.log
    grep '^FAIL: b$' test-suite.log
    grep '^b\.test: exit status: 23$' test-suite.log
    grep '^a\.test' test-suite.log && exit 1
    : For shells with busted 'set -e'.
  fi
  grep 'check-local failed :-(' local.log

  cd $srcdir

done

$MAKE distcheck

:
