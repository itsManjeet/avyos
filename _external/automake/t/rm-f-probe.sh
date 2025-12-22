#! /bin/sh
# Copyright (C) 2013-2025 Free Software Foundation, Inc.
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

# Verify our probe that checks that "rm -f" behavior works.
# https://bugs.gnu.org/10828

. test-init.sh

echo AC_OUTPUT >> configure.ac
: > Makefile.am

$ACLOCAL
$AUTOCONF
$AUTOMAKE

mkdir bin
cat > bin/rm <<'END'
#!/bin/sh
set -e; set -u;
PATH=$original_PATH; export PATH
rm_opts=
while test $# -gt 0; do
  case $1 in
    -*) rm_opts="$rm_opts $1";;
     *) break;;
  esac
  shift
done
if test $# -eq 0; then
  echo "Oops, fake rm called without arguments" >&2 #DELETE
  exit 1 #CHANGE
else
  exec rm $rm_opts "$@"
fi
END
chmod a+x bin/rm

original_PATH=$PATH
PATH=$(pwd)/bin$PATH_SEPARATOR$PATH
export PATH original_PATH

rm -f && exit 99 # Sanity check.

# Check that `rm -f` is detected as broken.
./configure
grep '^am__rm_f_notfound = ""$' Makefile

# Change the `rm -f` behavior to work.
sed -e '/#DELETE/d' -e '/#CHANGE/s:1:0:' bin/rm > bin/rm.tmp
cat bin/rm.tmp > bin/rm

# Check that `rm -f` is detected as working.
./configure
grep '^am__rm_f_notfound = *$' Makefile

# For the sake of our exit trap.
PATH=$original_PATH; export PATH

:
