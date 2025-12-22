#! /usr/bin/perl

# Build some of the Autoconf test files.

# Copyright (C) 2000-2017, 2020-2023 Free Software Foundation, Inc.

# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.

# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.

# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

use 5.006;
use strict;
use warnings FATAL => 'all';

# ac_exclude_list
# ---------------
# Check all AC_DEFUN'ed macros with AT_CHECK_MACRO, except these.
# Not every macro can be run without arguments, and some are already
# tested elsewhere.
my @ac_exclude_list = (
  # Internal macros should not be invoked directly from a configure.ac.
  qr/^_?_AC_/,

  # Used in many places.
  qr/^AC_.*_IFELSE$/,
  qr/^AC_LANG/,
  qr/^AC_RUN_LOG$/,
  qr/^AC_TRY/,

  # Need an argument.
  qr/^AC_(CANONICALIZE|PREFIX_PROGRAM|PREREQ)$/,
  qr/^AC_(SEARCH_LIBS|REPLACE_FUNCS)$/,
  qr/^AC_(CACHE_CHECK|COMPUTE)_INT$/,
  qr/^AC_ARG_VAR$/,
  qr/^AC_REQUIRE_SHELL_FN$/,

  # Performed in the semantics tests.
  qr/^AC_CHECK_(
        ALIGNOF|DECL|FILE|FUNC|HEADER|LIB|MEMBER
       |PROG|SIZEOF|(TARGET_)?TOOL|TYPE
     )S?$/x,
  qr/^AC_PATH_PROGS_FEATURE_CHECK$/,

  # Fail when the source does not exist.
  qr/^AC_CONFIG/,

  # AU defined to nothing.
  qr/^AC_(CYGWIN|CYGWIN32|EMXOS2|MING32|EXEEXT|OBJEXT)$/,

  # Produce "= val" because $1, the variable used to store the result,
  # is empty.
  qr/^AC_(F77|FC)_FUNC$/,
  qr/^AC_FC_(PP_)?SRCEXT$/,
  qr/^AC_PATH_((TARGET_)?TOOL|PROG)S?$/,

  # Is a number.
  qr/^AC_FD_CC$/,

  # Obsolete, but needs to be AC_DEFUNed and cannot safely be called
        # without arguments.  Tested in tools.at.
  qr/^AC_(DIAGNOSE|FATAL|FOREACH|OBSOLETE|WARNING)$/,

  # Require a file that is not shipped with Autoconf.  But it should.
  qr/^AC_FUNC_(GETLOADAVG|FNMATCH_GNU)$/,
  qr/^AC_REPLACE_FNMATCH$/,

  # Obsolete, checked in semantics.
  qr/^AC_FUNC_WAIT3$/,
  qr/^AC_FUNC_SETVBUF_REVERSED$/,
  qr/^AC_SYS_RESTARTABLE_SYSCALLS$/,

  # Not intended to be invoked at the top level.
  qr/^AC_INCLUDES_DEFAULT$/,

  # AC_INIT includes all the AC_INIT macros.
  # There is an infinite m4 recursion if AC_INIT is used twice.
  qr/^AC_INIT/,

  # Checked in semantics.
  qr/^AC_(PROG_CC|C_CONST|C_VOLATILE)$/,
  qr/^AC_PATH_XTRA$/,
  qr/^AC_SYS_(LARGEFILE|YEAR2038(_RECOMMENDED)?)$/,

  # Use without an argument is obsolete.
  # Checked in semantics.
  qr/^AC_PROG_LEX$/,

  # Requires a working C++ compiler, which is not a given.
  qr/^AC_PROG_CXX_C_O$/,

  # Already tested by AT_CHECK_MACRO.
  qr/^AC_OUTPUT$/,

  # Tested alongside m4_divert_text.
  qr/^AC_PRESERVE_HELP_ORDER$/,

  # Tested in erlang.at.
  qr/^AC_ERLANG_SUBST_(INSTALL_LIB_SUBDIR|ROOT_DIR)$/,
  qr/^AC_ERLANG_CHECK_LIB$/,
);

# au_exclude_list
# ---------------
# Check all AU_DEFUN'ed macros with AT_CHECK_AU_MACRO, except these.
my @au_exclude_list = (
  # Empty.
  qr/^AC_(C_CROSS|PROG_(CC_(C[89]9|STDC)|GCC_TRADITIONAL))$/,

  # Use AC_REQUIRE.
  qr/^AC_(CYGWIN|MINGW32|EMXOS2)$/,

  # Already in configure.ac.
  qr/^AC_(INIT|OUTPUT)$/,

  # AC_LANG_SAVE needs user interaction to be removed.
  # AC_LANG_RESTORE cannot be used alone.
  qr/^AC_LANG_(SAVE|RESTORE)$/,

  # Need arguments.  Tested in tools.at.
  qr/^AC_(DIAGNOSE|FATAL|OBSOLETE|WARNING)$/,
  qr/^AC_(FOREACH|LINK_FILES|PREREQ)$/,

  # Need arguments.  Tested in semantics.at.
  qr/^AC_HAVE_LIBRARY$/,
  qr/^AC_COMPILE_CHECK$/,
  qr/^AC_TRY_(COMPILE|CPP|LINK|RUN)$/,

  # Not macros, just mapping from old variable name to a new one.
  qr/^ac_cv_prog_(gcc|gxx|g77)$/,
);


# test_parameters
# ---------------
# Extra arguments to pass to the test macro for particular macros.
# Keys are macro names, values are records containing one or more
# of the possible optional arguments to AT_CHECK_(AU_)MACRO:
# macro_use, additional_commands, autoconf_flags, test_parameters => '...'.
# Entries in this hash are grouped by common situations, and sorted
# alphabetically within each group.
# Note that you must provide M4 quotation; emit_test will not quote
# the arguments for you.  (This is so you can _not_ quote the arguments
# when that's useful.)

my %test_parameters = (
  # Uses AC_RUN_IFELSE, cross-compilation test fails.
  AC_FC_CHECK_BOUNDS      => { test_parameters => '[no-cross]' },
  AC_FUNC_CHOWN           => { test_parameters => '[no-cross]' },
  AC_FUNC_FNMATCH         => { test_parameters => '[no-cross]' },
  AC_FUNC_FORK            => { test_parameters => '[no-cross]' },
  AC_FUNC_GETGROUPS       => { test_parameters => '[no-cross]' },
  AC_FUNC_LSTAT           => { test_parameters => '[no-cross]' },
  AC_FUNC_MALLOC          => { test_parameters => '[no-cross]' },
  AC_FUNC_MEMCMP          => { test_parameters => '[no-cross]' },
  AC_FUNC_MKTIME          => { test_parameters => '[no-cross]' },
  AC_FUNC_MMAP            => { test_parameters => '[no-cross]' },
  AC_FUNC_REALLOC         => { test_parameters => '[no-cross]' },
  AC_FUNC_STAT            => { test_parameters => '[no-cross]' },
  AC_FUNC_STRCOLL         => { test_parameters => '[no-cross]' },
  AC_FUNC_STRNLEN         => { test_parameters => '[no-cross]' },
  AC_FUNC_STRTOD          => { test_parameters => '[no-cross]' },

  # Different result with a C++ compiler than a C compiler:
  # C++ compilers may or may not support these features from C1999 and later.
  AC_C_RESTRICT => {
    test_parameters => ('[cxx_define_varies:restrict' .
                        ' cxx_cv_varies:cxx_restrict]')
  },
  AC_C_TYPEOF => {
    test_parameters => ('[cxx_define_varies:typeof' .
                        ' cxx_define_varies:HAVE_TYPEOF' .
                        ' cxx_cv_varies:cxx_typeof]')
  },
  AC_C__GENERIC => {
    test_parameters => ('[cxx_define_varies:HAVE_C__GENERIC' .
                        ' cxx_cv_varies:cxx__Generic]')
  },
  AC_C_VARARRAYS => {
    test_parameters => ('[cxx_define_varies:HAVE_C_VARARRAYS' .
                        ' cxx_define_varies:__STDC_NO_VLA__' .
                        ' cxx_cv_varies:cxx_vararrays]')
  },

  # stdbool.h is supposed to be includable from C++, per C++2011
  # [support.runtime], but the type _Bool was not added to the C++
  # standard, so it may or may not be present depending on how much
  # the C++ compiler cares about C source compatibility.
  AC_CHECK_HEADER_STDBOOL => {
    test_parameters => ('[cxx_define_varies:HAVE__BOOL' .
                        ' cxx_cv_varies:type__Bool]')
  },
  AC_HEADER_STDBOOL => {
    test_parameters => ('[cxx_define_varies:HAVE__BOOL' .
                        ' cxx_cv_varies:type__Bool]')
  },

  # G++ forces -D_GNU_SOURCE which, with some versions of GNU libc,
  # changes the declaration of strerror_r.  Blech.
  AC_FUNC_STRERROR_R => {
    test_parameters => ('[cxx_define_varies:STRERROR_R_CHAR_P' .
                        ' cxx_cv_varies:func_strerror_r_char_p]')
  },
);

# skip_macro MACRO, EXCLUDE-LIST
# ------------------------------
# Returns truthy if any of the regexes in EXCLUDE-LIST match MACRO.
sub skip_macro
{
  my $macro = shift;
  for my $pat (@_)
    {
      return 1 if $macro =~ $pat;
    }
  return 0;
}


# emit_test FH, TEST-MACRO, MACRO
# --------------------------------
# Emit code to FH to test MACRO using TEST-MACRO.
# TEST-MACRO is expected to be either AT_CHECK_MACRO or AT_CHECK_AU_MACRO;
# see local.at.
sub emit_test
{
  my ($fh, $test_macro, $macro) = @_;
  my $params = $test_parameters{$macro} || {};
  my $macro_use           = ${$params}{macro_use}           || '';
  my $additional_commands = ${$params}{additional_commands} || '';
  my $autoconf_flags      = ${$params}{autoconf_flags}      || '';
  my $test_parameters     = ${$params}{test_parameters}     || '';

  $autoconf_flags = '[]'
    if $autoconf_flags eq '' && $test_parameters ne '';
  $additional_commands = '[]'
    if $additional_commands eq '' && $autoconf_flags ne '';
  $macro_use = '[]'
    if $macro_use eq '' && $additional_commands ne '';

  print $fh "$test_macro([$macro]";
  print $fh ", $autoconf_flags" if $autoconf_flags ne '';
  print $fh ", $additional_commands" if $additional_commands ne '';
  print $fh ", $autoconf_flags" if $autoconf_flags ne '';
  print $fh ", $test_parameters" if $test_parameters ne '';
  print $fh ")\n";
}

# scan_m4_files
# -------------
# Scan all of the Autoconf source files and produce lists of macros
# to be tested
sub scan_m4_files
{
  my @macros_to_test;
  my %required_macros;

  for my $file (@_)
    {
      open my $fh, "<", $file
        or die "$file: $!\n";

      my (@ac_macros, @au_macros);
      while (<$fh>)
        {
          chomp;
          s/\bdnl\b.*$//;
          if (/\bAC_REQUIRE\(\[*([a-zA-Z0-9_]+)/)
            {
              $required_macros{$1} = 1;
            }
          elsif (/^AC_DEFUN(?:_ONCE)?\(\[*([a-zA-Z0-9_]+)/)
            {
              push @ac_macros, $1
                unless skip_macro $1, @ac_exclude_list;
            }
          elsif (/^AU_DEFUN?\(\[*([a-zA-Z0-9_]+)/)
            {
              push @au_macros, $1
                unless skip_macro $1, @au_exclude_list;
            }
        }
      push @macros_to_test, [ $file, \@ac_macros, \@au_macros ];
    }

  # Do **NOT** filter out AC_FUNC_ALLOCA. Filtering it out
  # ended up eliding a direct test of AC_FUNC_ALLOCA which
  # would have exposed a bug, while no required use does so.
  # Clearing this hash entirely would currently enable direct tests
  # of 38 macros, but would require designating each that must be
  # skipped when cross-compiling.
  delete $required_macros{AC_FUNC_ALLOCA};

  # Filter out macros that are AC_REQUIREd by some other macro;
  # it's not necessary to test them directly.
  my @pruned_macros_to_test;
  for my $elt (@macros_to_test)
    {
      my ($file, $ac_macros, $au_macros) = @$elt;
      my (@pruned_ac_macros, @pruned_au_macros);

      for my $macro (@$ac_macros)
        {
          push @pruned_ac_macros, $macro
            unless defined $required_macros{$macro};
        }
      for my $macro (@$au_macros)
        {
          push @pruned_au_macros, $macro
            unless defined $required_macros{$macro};
        }

      push @pruned_macros_to_test, [
        $file, \@pruned_ac_macros, \@pruned_au_macros
      ];
    }

  return @pruned_macros_to_test;
}


# make_read_only FILE
# -------------------
# Make FILE read-only on disk.  Also clears the execute and special bits.
sub make_read_only
{
  my $f = shift;
  my $mode = (stat $f)[2];
  die "stat($f): $!\n" unless defined $mode;
  # clear all the bits in $mode except r--r--r--
  $mode &= 00444;
  chmod $mode, $f or die "making $f read-only: $!\n";
}


# create_test_files OUTDIR, M4-FILES...
# -----------------
# Main loop: for each file listed in M4-FILES, generate an .at file
# named "ac${file}.at" that does cursory tests on each of the macros
# defined in $file.
sub create_test_files
{
  my $outdir = shift;
  if (@_ == 0)
    {
      print STDERR "usage: $0 outdir m4-files...\n";
      exit 1;
    }

  for my $elt (scan_m4_files @_)
    {
      my ($file, $ac_macros, $au_macros) = @$elt;

      my $base = $file;
      $base =~ s|^.*/([^/.]+)(?:.[^/]*)?$|$1|;

      my $tmpout = "${outdir}/ac${base}.tat";
      my $out = "${outdir}/ac${base}.at";

      open my $fh, ">", $tmpout
        or die "$tmpout: $!\n";

      print $fh <<"EOF";
# -*- autotest -*-
# Generated by $0 from $file.
# Do not edit this file by hand.

EOF

      if (@$ac_macros || @$au_macros)
        {
          print $fh "AT_BANNER([Testing autoconf/$base macros.])\n";

          if (@$ac_macros)
            {
              print $fh "\n# Modern macros.\n";
              emit_test ($fh, 'AT_CHECK_MACRO', $_)
                for sort @$ac_macros;
            }
          if (@$au_macros)
            {
              print $fh "\n# Obsolete macros.\n";
              emit_test ($fh, 'AT_CHECK_AU_MACRO', $_)
                for sort @$au_macros;
            }
        }
      else
        {
          print $fh "# Nothing to test.\n";
        }

      close $fh or die "writing to $tmpout: $!\n";
      make_read_only $tmpout;
      rename $tmpout, $out or die "updating $out: $!\n";
    }
}

create_test_files @ARGV;
exit 0;

### Setup "GNU" style for perl-mode and cperl-mode.
## Local Variables:
## perl-indent-level: 2
## perl-continued-statement-offset: 2
## perl-continued-brace-offset: 0
## perl-brace-offset: 0
## perl-brace-imaginary-offset: 0
## perl-label-offset: -2
## cperl-indent-level: 2
## cperl-brace-offset: 0
## cperl-continued-brace-offset: 0
## cperl-label-offset: -2
## cperl-extra-newline-before-brace: t
## cperl-merge-trailing-else: nil
## cperl-continued-statement-offset: 2
## End:
