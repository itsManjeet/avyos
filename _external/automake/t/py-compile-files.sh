#! /bin/sh
# Copyright (C) 2022-2025 Free Software Foundation, Inc.
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

# Verify 'py-compile' script can handle inputs with spaces, etc...

required=python
. test-init.sh

cp "$am_scriptdir/py-compile" . \
  || fatal_ "failed to fetch auxiliary script py-compile"

# Create files that require proper quoting.
mkdir "dir with spaces"
touch "nospace.py" "has space.py" "*.py" "dir with spaces/|.py"

./py-compile "nospace.py" "has space.py" "*.py" "dir with spaces/|.py"

py_installed "nospace.pyc"
py_installed "has space.pyc"
py_installed "*.pyc"
py_installed "dir with spaces/|.pyc"

:
