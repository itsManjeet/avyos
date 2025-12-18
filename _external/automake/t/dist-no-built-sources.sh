#! /bin/sh
# Copyright (C) 2021-2025 Free Software Foundation, Inc.
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

# Test the presence and absence of the option no-dist-built-sources.

. test-init.sh

# the tests are almost the same, so do a loop with a couple conditionals.
# 
# test-init.sh creates configure.ac with an AM_INIT_AUTOMAKE call with
# no options. The default is [no-no-]dist-built-sources, i.e., distdir
# does depend on $(BUILT_SOURCES), so test that first. (There is no
# Automake option named dist-built-sources, only --no-no-dist-built-sources.)
# 
# The second time around, add the no-dist-built-sources option,
# and the distdir target should not depend on anything.
#
for testopt in dist-built-sources no-dist-built-sources; do

  if test "$testopt" = no-dist-built-sources; then
    sed -e 's/AM_INIT_AUTOMAKE/AM_INIT_AUTOMAKE([no-dist-built-sources])/' \
        configure.ac >configure.tmp
    cmp configure.ac configure.tmp \
    && fatal_ 'failed to edit configure.ac for dist-built-sources'
    mv -f configure.tmp configure.ac
  fi

#printf "\n\f test=$testopt, configure.ac is:\n"
#cat configure.ac

  cat >> configure.ac << 'END'
AC_OUTPUT
END

  cat > Makefile.am <<EOF
BUILT_SOURCES = x.c
EXTRA_DIST = y.c

x.c:
	touch \$@

y.c:
	cp x.c y.c # simulate 'undetectable' dependency on x.c
EOF

  if test "$testopt" = no-dist-built-sources; then
    touch y.c # no-dist-built-sources dist needs to have all files already
  else
    : # with default $(BUILT_SOURCES) dep, will try to build y.c by the rule
  fi

  $ACLOCAL
  $AUTOMAKE
  $AUTOCONF
  ./configure
  run_make dist

#printf "\n\f test=$testopt, Makefile has:\n"
#grep ^distdir: Makefile

  # In any case, the tarball should contain y.c, but not x.c.
  # The tarball is always named based on $0, regardless of our options.
  pkg_ver=$me-1.0
  gzip -d "${pkg_ver}".tar.gz
  ! tar tf "${pkg_ver}".tar "${pkg_ver}"/x.c
  tar tf "${pkg_ver}".tar "${pkg_ver}"/y.c

  # And x.c should have been built only for the built-sources version.
  if test "$testopt" = no-dist-built-sources; then
    # no-built-sources should not have generated this
    ! test -e x.c
    grep 'distdir:$' Makefile
  else
    # built-sources build should have it
    test -e x.c
    grep 'distdir: \$(BUILT_SOURCES)' Makefile
  fi

  # If the test runs fast enough, the make dist second time through
  # won't do anything since the tarball will be considered up to date.
  rm -f "${pkg_ver}".tar.gz "${pkg_ver}".tar
done

:
