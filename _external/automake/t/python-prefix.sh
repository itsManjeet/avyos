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

# Test configure options --with-python_prefix and --with-python_exec_prefix.

required=python
. test-init.sh

cat >>configure.ac <<EOF
AM_PATH_PYTHON
AC_OUTPUT
EOF

cat >Makefile.am <<'END'
# to be installed in pythondir:
python_PYTHON = one.py

# to be installed in pythonpkgdir:
pkgpython_PYTHON = pkgtwo.py

one.py:
	echo 'def one(): return 1' >$@ || rm -f $@
pkgtwo.py:
	echo 'def pkgtwo(): return 1' >$@ || rm -f $@

# It's too much trouble to build and install something that actually
# needs to be under exec_prefix. Instead, we'll just check the value of
# the variable.
echo-python-exec-prefix:
	@echo $(PYTHON_EXEC_PREFIX)
END

if test -z "$PYTHON"; then
  py_exec=python
else
  py_exec=$PYTHON
fi
py_version=$("$py_exec" -c 'import sys; print("%u.%u" % sys.version_info[:2])')
py_inst_site=inst/lib/python$py_version/site-packages
py_instexec_site=instexec/lib/python$py_version/site-packages

#  First test: if --with-python_prefix is given, by default it should
# be used for python_exec_prefix too.
#
$ACLOCAL
$AUTOCONF
$AUTOMAKE --add-missing

mkdir build
cd build
../configure --with-python_prefix="$(pwd)/inst"
$MAKE install
#
py_installed "$py_inst_site"/one.py
py_installed "$py_inst_site"/one.pyc
#
py_installed "$py_inst_site"/python-prefix/pkgtwo.py
py_installed "$py_inst_site"/python-prefix/pkgtwo.pyc
#
test "$($MAKE echo-python-exec-prefix)" = "$(pwd)/inst"

#  Second test: specify different --with-python_prefix
# and --with-python_exec_prefix values.
#
cd ..
rm -rf build auto4mte.cache
mkdir build
cd build
../configure --with-python_prefix="$(pwd)/inst" \
             --with-python_exec_prefix="$(pwd)/instexec"
$MAKE install
#
py_installed "$py_inst_site"/one.py
py_installed "$py_inst_site"/one.pyc
#
py_installed "$py_inst_site"/python-prefix/pkgtwo.py
py_installed "$py_inst_site"/python-prefix/pkgtwo.pyc
#
test "$($MAKE echo-python-exec-prefix)" = "$(pwd)/instexec"

:
