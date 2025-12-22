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

# Check that AM_PATH_PYTHON correctly sets all the output variables
# advertised in the manual, both with the GNU prefix values and the
# Python sys.* prefix values.

required=python
. test-init.sh

# In case the user's config.site defines pythondir or pyexecdir.
CONFIG_SITE=/dev/null; export CONFIG_SITE

# Update the definition below if the documentation changes. The values
# of the 'pythondir' and 'pyexecdir' variables vary among different
# python installations, so we need more relaxed and ad-hoc checks for
# them. Also, more proper "functional" checks on them should be done in
# the 'python-virtualenv.sh' test.
#
# This version identification is duplicated in python.m4 (and the manual).
PYTHON_VERSION=$($PYTHON -c 'import sys; print ("%u.%u" % sys.version_info[:2])') || exit 1
PYTHON_PLATFORM=$($PYTHON -c 'import sys; print (sys.platform)') || exit 1
PYTHON_EXEC_PREFIX=$($PYTHON -c 'import sys; print (sys.exec_prefix)') || exit 1
PYTHON_PREFIX=$($PYTHON -c 'import sys; print (sys.prefix)') || exit 1
pkgpythondir="\${pythondir}/$me"
pkgpyexecdir="\${pyexecdir}/$me"

pyvars='PYTHON_VERSION PYTHON_PLATFORM PYTHON_PREFIX PYTHON_EXEC_PREFIX
        pkgpythondir pkgpyexecdir'

cat >> configure.ac << 'END'
AC_CONFIG_FILES([vars-got pythondir pyexecdir])
AM_PATH_PYTHON
AC_OUTPUT
END

cat > my.py << 'END'
def my():
    return 1
END

cat > Makefile.am << 'END'

python_PYTHON = my.py

EXTRA_DIST = vars-exp

check-local: test-in test-am
.PHONY: test-in test-am

test-in:
	@echo "> doing test-in"
	@echo ">> contents of pythondir:"
	cat pythondir
	case `cat pythondir` in '$${PYTHON_PREFIX}'/*);; *) exit 1;; esac
	@echo ">> contents of pyexecdir:"
	cat pyexecdir
	case `cat pyexecdir` in '$${PYTHON_EXEC_PREFIX}'/*);; *) exit 1;; esac
	@echo ">> contents of vars-exp:"
	cat $(srcdir)/vars-exp
	@echo ">> contents of vars-got:"
	cat $(builddir)/vars-got
	diff $(srcdir)/vars-exp $(builddir)/vars-got

## Note: this target's rules will be extended in the "for" loop below.
test-am:
	@echo "> doing test-am"
	case '$(pythondir)' in '$(PYTHON_PREFIX)'/*);; *) exit 1;; esac
	case '$(pyexecdir)' in '$(PYTHON_EXEC_PREFIX)'/*);; *) exit 1;; esac
END

echo @pythondir@ > pythondir.in
echo @pyexecdir@ > pyexecdir.in

# This depends on whether we're doing GNU or Python values, per arg.
setup_vars_file ()
{
  vartype=$1
  : > vars-exp
  : > vars-got.in

  for var in $pyvars; do
    if test x"$vartype" = xgnu; then
      # when not using Python sys.* values, PYTHON_*PREFIX will vary;
      # the computed value will be (something like) "/usr",
      # but the expected value will be "${prefix}".
      if test x"$var" = xPYTHON_PREFIX \
         || test x"$var" = xPYTHON_EXEC_PREFIX; then
        continue
      fi
    fi
    eval val=\$$var
    echo "var=$val  #$var"   >> vars-exp
    echo "var=@$var@  #$var" >> vars-got.in
    echo "${tab}test x'\$($var)' = x'$val' || test \"\$NO_CHECK_PYTHON_PREFIX\"" >> Makefile.am
  done
}

setup_vars_file gnu

$ACLOCAL
$AUTOMAKE --add-missing

# some debugging output.
for var in pythondir pyexecdir $pyvars; do
  grep "^$var *=" Makefile.in
done

$AUTOCONF

# Do GNU values.
./configure PYTHON="$PYTHON"
$MAKE test-in test-am
run_make distcheck

# Do Python values.
setup_vars_file python
instdir=$(pwd)/inst
./configure PYTHON="$PYTHON" --with-python-sys-prefix --prefix="$instdir"
$MAKE test-in test-am
#
# This tries to install to $PYTHON_PREFIX, which may not be writable.
# Override it to something safe, but then of course we have to skip
# checking that it is what we originally set it to.
run_make distcheck \
  PYTHON_PREFIX="$instdir" \
  NO_CHECK_PYTHON_PREFIX=1 \
  AM_DISTCHECK_CONFIGURE_FLAGS=--with-python-sys-prefix

:
