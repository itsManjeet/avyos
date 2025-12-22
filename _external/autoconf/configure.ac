#                                                       -*- Autoconf -*-
# Process this file with autoconf to produce a configure script.

# Copyright (C) 1992-1995, 1999-2017, 2020-2023 Free Software
# Foundation, Inc.

# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

AC_PREREQ([2.69])

# Keep both of these on a single line, and do not use quadrigraphs
# or nested quotes in their values, since the bootstrap script needs
# to be able to extract their values with sed.
m4_define([autoconf_PACKAGE_NAME], [GNU Autoconf])
m4_define([autoconf_PACKAGE_BUGREPORT], [bug-autoconf@gnu.org])

AC_INIT(
  m4_defn([autoconf_PACKAGE_NAME]),
  [m4_esyscmd([build-aux/git-version-gen .tarball-version])],
  m4_defn([autoconf_PACKAGE_BUGREPORT])
)
AC_CONFIG_SRCDIR([lib/autoconf/autoconf.m4])

AC_CONFIG_AUX_DIR([build-aux])

AM_INIT_AUTOMAKE([1.11 dist-xz readme-alpha no-texinfo.tex std-options])

# Keep this on a line of its own, since it must be found and processed
# by the 'update-release-year' rule in our Makefile.
RELEASE_YEAR=2023
AC_SUBST([RELEASE_YEAR])

AB_INIT

# We use 'sh -n script' to check that there are no syntax errors
# in the scripts.  Although incredible, there are sh implementations
# that go into endless loops with '-n', e.g., SunOS's:
#
#   $ uname -a
#   SunOS ondine 4.1.3 2 sun4m unknown
#   $ cat endless.sh
#   while false
#   do
#     :
#   done
#   exit 0
#   $ time sh endless.sh
#   sh endless.sh  0,02s user 0,03s system 78% cpu 0,064 total
#   $ time sh -nx endless.sh
#   ^Csh -nx endless.sh  3,67s user 0,03s system 63% cpu 5,868 total
#
# Also, some implementations (e.g., Solaris 8) are soooo slow
# that they are unusable on large scripts like our testsuite.
#
# So we must identify a shell whose -n can safely be used.

AC_CACHE_CHECK([for a shell whose -n mode is known to work],
               [ac_cv_sh_working_n],
[ac_cv_sh_working_n=none
# Start by trying the shell that autoconf decided to use for this script,
# follow with a hardwired list of shells that are known to work and can
# be identified as such, starting with the ones with the fewest
# syntactic extensions.  Unfortunately, several shells that are also
# known to work can't be easily identified (e.g. some BSD shells and dash).
# Try ksh93, which is often buggy, and plain ksh and sh last.
for cand_sh in "$SHELL" pdksh bash zsh ksh93 ksh sh
do
  if (
    unset BASH_VERSION ZSH_VERSION
    "$cand_sh" -c '
      test ${BASH_VERSION+y} || # Bash
      test ${KSH_VERSION+y}  || # pdksh
      test ${ZSH_VERSION+y}  || # zsh
      test ${NETBSD_SHELL+y} || # NetBSD sh
      test -n "${.sh.version}" # ksh93; put this last since its syntax is dodgy
    '
  ) 2>/dev/null
  then
    ac_cv_sh_working_n="$cand_sh"
    break
  fi
done
])
AC_SUBST([SHELL_N], [$ac_cv_sh_working_n])

AC_MSG_CHECKING([for characters that cannot appear in file names])
AC_CACHE_VAL([ac_cv_unsupported_fs_chars],
[ac_cv_unsupported_fs_chars=
for c in '\\' \" '<' '>' '*' '?' '|'
do
  touch "conftest.t${c}t" 2>/dev/null
  test -f "conftest.t${c}t" && rm -f "conftest.t${c}t" && continue
  # $c cannot be used in a file name.
  ac_cv_unsupported_fs_chars=$ac_cv_unsupported_fs_chars$c
done
])
if test -n "$ac_cv_unsupported_fs_chars"; then
  AC_MSG_RESULT([$ac_cv_unsupported_fs_chars])
else
  AC_MSG_RESULT([none])
fi

AC_SUBST([ac_cv_unsupported_fs_chars])

AC_CACHE_CHECK([whether directories can have trailing spaces],
	       [ac_cv_dir_trailing_space],
[rm -rf 'conftest.d ' && mkdir 'conftest.d ' && touch 'conftest.d /tfile' 2>/dev/null
stat=$?
rm -rf 'conftest.d '
case $stat$? in #(
   00) ac_cv_dir_trailing_space=yes ;; #(
   *) ac_cv_dir_trailing_space=no ;;
esac
])
AC_SUBST([ac_cv_dir_trailing_space])

# Initialize the test suite.
AC_CONFIG_TESTDIR([tests])
AC_CONFIG_FILES([tests/atlocal])
AC_PATH_PROG([EXPR], [expr])


## ---- ##
## M4.  ##
## ---- ##

# We use an absolute name for GNU m4 so even if users have another m4 first in
# their path, the installer can configure with a path that has GNU m4
# on it and get that path embedded in the installed autoconf and
# autoheader scripts.
AC_PROG_GNU_M4

## ----------- ##
## Man pages.  ##
## ----------- ##

AM_MISSING_PROG([HELP2MAN], [help2man])


## ------ ##
## Perl.  ##
## ------ ##

# We use an absolute name for perl so the #! line in autoscan will work.
AC_PATH_PROG([PERL], [perl], [no])
AC_SUBST([PERL])dnl
if test "$PERL" = no; then
  AC_MSG_ERROR([perl is not found])
fi
$PERL -e 'require 5.006;' || {
   AC_MSG_ERROR([Perl 5.006 or better is required])
}

# Find out whether the system supports flock
# Note this test does not try to find out whether it works with this
# particular file system.  It merely avoids us running flock on systems
# where that bails out.
AC_CACHE_CHECK([whether $PERL Fcntl::flock is implemented],
  [ac_cv_perl_flock_implemented],
  [echo lock me > conftest.fil
   if $PERL -e 'use Fcntl ":flock"; flock("conftest.fil", LOCK_EX); 1;'; then
     ac_cv_perl_flock_implemented=yes
   else
     ac_cv_perl_flock_implemented=no
   fi
   rm -f conftest.fil
  ])
AC_SUBST([PERL_FLOCK], [$ac_cv_perl_flock_implemented])


## ------- ##
## Emacs.  ##
## ------- ##

TEST_EMACS=$EMACS
test x"$TEST_EMACS" = xt && TEST_EMACS=
AC_CHECK_PROGS([TEST_EMACS], [emacs xemacs], [no])

# autoconf-mode.el and autotest-mode.el do not work with older version of
# Emacs (i.e. 18.x.x).  During byte-compilation, Emacs complains:
# "Variable (broken nil) seen on pass 2 of byte compiler but not pass 1"
# We detect this problem here.
AS_IF([test "$TEST_EMACS" != no],
  [AC_CACHE_CHECK([whether $TEST_EMACS is sufficiently recent],
    [ac_cv_prog_emacs_ok],
    [# Note: The quoted "EOF" is intentional.  It protects the ` in the text.
    cat >conftest.el << "EOF"
      (defvar broken)
      (defun broken-mode () (setq broken-defaults `(broken nil)))
EOF
    AS_IF([AC_RUN_LOG(
      [$TEST_EMACS -batch -q -f batch-byte-compile conftest.el 1>&2])],
      [ac_cv_prog_emacs_ok=yes], [ac_cv_prog_emacs_ok=no])
    rm -f conftest.el conftest.elc])
  AS_IF([test $ac_cv_prog_emacs_ok = no], [TEST_EMACS=no])])

AC_SUBST([EMACS], [$TEST_EMACS])

AM_PATH_LISPDIR


## ---------------- ##
## Grep, sed, awk.  ##
## ---------------- ##
AC_PROG_GREP
AC_PROG_EGREP
AC_PROG_SED
# AC_PROG_AWK should have already been performed by AM_INIT_AUTOMAKE.
# AC_REQUIRE([AC_PROG_AWK]) doesn't work outside of an AC_DEFUN.
m4_provide_if([AC_PROG_AWK], [], [AC_PROG_AWK])


## ----- ##
## Make. ##
## ----- ##
AC_PROG_MAKE_CASE_SENSITIVE


## ------------ ##
## Conclusion.  ##
## ------------ ##

dnl Allow maintainer rules under GNU make even in VPATH builds.
AC_CONFIG_LINKS([GNUmakefile:GNUmakefile])

AC_CONFIG_FILES([Makefile])
AC_CONFIG_FILES([lib/version.m4:lib/version.in])

AC_OUTPUT
# Report the state of this version of Autoconf if this is a beta.
m4_bmatch(m4_defn([AC_PACKAGE_VERSION]), [[-a-z]],
[    cat <<EOF

You are about to use an experimental version of Autoconf.  Be sure to
read the relevant mailing lists, most importantly <autoconf@gnu.org>.

Below you will find information on the status of this version of Autoconf.

EOF
    sed -n '/^\* Status/,$p' $srcdir/BUGS
echo
])dnl
