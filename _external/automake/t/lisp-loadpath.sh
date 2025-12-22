#! /bin/sh
# Copyright (C) 2012-2025 Free Software Foundation, Inc.
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

# Emacs lisp files in both $(srcdir) and $(builddir) are found if
# required by other files.  Related to automake bug#11806.

required=emacs
. test-init.sh

# The story here is that at least in Emacs 21, -L foo -L bar ends up
# with bar before foo in load-path. The invocation in the .el.elc rule
# in lisp.am correctly uses -L $(builddir) -L $(srcdir), and thus the
# test below ends up failing. So skip the test on such old Emacs; no
# need to work around in the code.
#
# At least as of Emacs 24, -L foo -L bar preserves command line order,
# so foo is before bar in load-path, and all is well.
#
# Situation with Emacs 22 and 23 is unknown, so play it safe and skip
# the test for them too.
#
# Meanwhile, Emacs sets the EMACS envvar to t in subshells.
# If that's what we've got, use "emacs" instead.
test "$EMACS" = t && EMACS=emacs || :

emacs_major=$(${EMACS-emacs} --version | sed -e 's/.* //;s/\..*$//;1q')
if test -z "$emacs_major" || test "$emacs_major" -le 23; then
  skip_ "emacs version $emacs_major may reverse -L ordering"
fi

cat >> configure.ac << 'END'
AM_PATH_LISPDIR
AC_OUTPUT
END

cat > Makefile.am << 'END'
noinst_LISP = requirer.el
lisp_LISP = foo.el
lisp_DATA = bar.el
END

echo "(require 'foo) (require 'bar)" >> requirer.el
echo "(provide 'foo)" > foo.el
echo "(provide 'bar)" > bar.el

$ACLOCAL
$AUTOCONF
$AUTOMAKE -a

mkdir build
cd build
../configure
$MAKE
test -f requirer.elc
test -f foo.elc
test ! -e bar.elc

$MAKE clean
test ! -e requirer.elc
test ! -e foo.elc

# In the spirit of VPATH, stuff in the builddir is preferred to
# stuff in the srcdir.
echo "(provide" > ../foo.el  # Break it.
echo "defun)" > ../bar.el    # Likewise.
$MAKE && exit 1
$sleep
echo "(provide 'foo)" > foo.el
echo "(provide 'bar)" > bar.el
$MAKE
test -f requirer.elc
test -f foo.elc
test ! -e bar.elc

:
