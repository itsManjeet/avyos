# help-extract -- extract --help and --version output from a script.
# Copyright (C) 2020-2023 Free Software Foundation, Inc.

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

# Written by Zack Weinberg.

use 5.010;
use strict;
use warnings;
use File::Spec::Functions qw(catfile);

# This script is not intended to be used directly.  It's run by
# help2man via wrappers in man/, e.g.  man/autoconf.w, as if it were
# one of autoconf's executable scripts.  It extracts the --help and
# --version output of that script from its source form, without
# actually running it.  The script to work from is set by the wrapper,
# and several other parameters are passed down from the Makefile as
# environment variables; see parse_args below.

# The point of this script is, the preprocessed forms of the
# executable scripts, and their wrappers for uninstalled use
# (e.g. <build-dir>/{bin,tests}/autoconf) do not need to exist to
# generate the corresponding manpages.  This is desirable because we
# can't put those dependencies in the makefiles without breaking
# people's ability to build autoconf from a release tarball without
# help2man installed.  It also ensures that we will generate manpages
# from the current source code and not from an older version of the
# script that has already been installed.

## ----------------------------- ##
## Extraction from Perl scripts. ##
## ----------------------------- ##

sub eval_qq_no_interpolation ($)
{
  # The argument is expected to be a "double quoted string" including the
  # leading and trailing delimiters.  Returns the text of this string after
  # processing backslash escapes but NOT interpolation.
  my $s = $_[0];

  # Escape $ and @ inside the string, if they are not already escaped.
  # The regex matches the empty string, but only if it is preceded by an
  # even number of backslashes (including zero) and followed by either a
  # literal $ or a literal @. Then we insert a backslash at the position
  # of the match.
  $s =~ s/ (?:\A|[^\\]) (?:\\\\)* \K (?=[\$\@]) /\\/xg;

  # It is now safe to feed the string to 'eval'.
  return eval $s;
}

sub extract_channeldefs_usage ($)
{
  my ($channeldefs_pm) = @_;
  my $usage = "";
  my $parse_state = 0;
  local $_;

  open (my $fh, "<", $channeldefs_pm) or die "$channeldefs_pm: $!\n";
  while (<$fh>)
    {
      if ($parse_state == 0)
	{
	  $parse_state = 1 if /^sub usage\b/;
	}
      elsif ($parse_state == 1)
	{
	  if (s/^  return "//)
	    {
	      $parse_state = 2;
	      $usage .= $_;
	    }
	}
      elsif ($parse_state == 2)
	{
	  if (s/(?<!\\) ((?>\\\\)*) "; $/$1/x)
	    {
	      $usage .= $_;
	      return $usage;
	    }
	  else
	    {
	      $usage .= $_;
	    }
	}
    }

  die "$channeldefs_pm: unexpected EOF in state $parse_state\n";
}

sub extract_perl_assignment (*$$$)
{
  my ($fh, $source, $channeldefs_pm, $what) = @_;
  my $value = "";
  my $parse_state = 0;
  local $_;

  while (<$fh>)
    {
      if ($parse_state == 0)
	{
	  if (s/^\$\Q${what}\E = (?=")//o)
	    {
	      $value .= $_;
	      $parse_state = 1;
	    }
	}
      elsif ($parse_state == 1)
	{
	  if (/^"\s*\.\s*Autom4te::ChannelDefs::usage\s*(?:\(\))?\s*\.\s*"$/)
	    {
	      $value .= extract_channeldefs_usage ($channeldefs_pm);
	    }
	  elsif (/^";$/)
	    {
	      $value .= '"';
	      return eval_qq_no_interpolation ($value);
	    }
	  else
	    {
	      $value .= $_;
	    }
	}
    }

  die "$source: unexpected EOF in state $parse_state\n";
}


## -------------- ##
## Main program.  ##
## -------------- ##

sub extract_assignment ($$$)
{
  my ($source, $channeldefs_pm, $what) = @_;

  open (my $fh, "<", $source) or die "$source: $!\n";

  my $firstline = <$fh>;
  if ($firstline =~ /\@PERL\@/ || $firstline =~ /-\*-\s*perl\s*-\*-/i)
    {
      return extract_perl_assignment ($fh, $source, $channeldefs_pm, $what);
    }
  else
    {
      die "$source: language not recognized\n";
    }
}

sub main ()
{
  # Most of our arguments come from environment variables, because
  # help2man doesn't allow for passing additional command line
  # arguments to the wrappers, and it's easier to write the wrappers
  # to not mess with the command line.
  my $usage = "Usage: $0 script-source (--help | --version)

Extract help and version information from a perl script.
Required environment variables:

  top_srcdir       relative path from cwd to the top of the source tree
  channeldefs_pm   relative path from top_srcdir to ChannelDefs.pm
  PACKAGE_NAME     the autoconf PACKAGE_NAME substitution variable
  VERSION          the autoconf VERSION substitution variable
  RELEASE_YEAR     the autoconf RELEASE_YEAR substitution variable

The script-source argument should also be relative to top_srcdir.
";

  my $source         = shift(@ARGV)         || die $usage;
  my $what           = shift(@ARGV)         || die $usage;
  my $top_srcdir     = $ENV{top_srcdir}     || die $usage;
  my $channeldefs_pm = $ENV{channeldefs_pm} || die $usage;
  my $package_name   = $ENV{PACKAGE_NAME}   || die $usage;
  my $version        = $ENV{VERSION}        || die $usage;
  my $release_year   = $ENV{RELEASE_YEAR}   || die $usage;

  if ($what eq "-h" || $what eq "--help")
    {
      $what = "help";
    }
  elsif ($what eq "-V" || $what eq "--version")
    {
      $what = "version";
    }
  else
    {
      die $usage;
    }

  my $cmd_name    = $source;
  $cmd_name       =~ s{^.*/([^./]+)\.in$}{$1};
  $source         = catfile($top_srcdir, $source);
  $channeldefs_pm = catfile($top_srcdir, $channeldefs_pm);

  my $text = extract_assignment ($source, $channeldefs_pm, $what);
  $text =~ s/\$0\b/$cmd_name/g;
  $text =~ s/[@]PACKAGE_NAME@/$package_name/g;
  $text =~ s/[@]VERSION@/$version/g;
  $text =~ s/[@]RELEASE_YEAR@/$release_year/g;
  print $text;
}

main;


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
