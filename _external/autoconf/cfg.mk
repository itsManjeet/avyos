# Customize maint.mk for Autoconf.            -*- Makefile -*-
# Copyright (C) 2003-2004, 2006, 2008-2017, 2020-2023 Free Software
# Foundation, Inc.

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

# This file is '-include'd into GNUmakefile.

# Build with our own versions of these tools, when possible.
export PATH := $(or $(PWD),$(shell pwd))/tests:$(PATH)

# Remove the autoreconf-provided INSTALL, so that we regenerate it.
_autoreconf = autoreconf -i -v && rm -f INSTALL

# Used in maint.mk's web-manual rule
manual_title = Creating Automatic Configuration Scripts

# The local directory containing the checked-out copy of gnulib used
# in this release (override the default).  The $GNULIB_SRCDIR variable
# is also honored by the gnulib-provided bootstrap script, so using it
# here is consistent.
gnulib_dir = $${GNULIB_SRCDIR-'$(abs_srcdir)'/../gnulib}

# The bootstrap tools (override the default).
bootstrap-tools = automake

# Set preferred lists for announcements.

announcement_Cc_ = $(PACKAGE_BUGREPORT), autotools-announce@gnu.org
announcement_mail-alpha = autoconf@gnu.org
announcement_mail-beta = autoconf@gnu.org
announcement_mail-stable = info-gnu@gnu.org, autoconf@gnu.org
announcement_mail_headers_ =						\
To: $(announcement_mail-$(RELEASE_TYPE))				\
CC: $(announcement_Cc_)							\
Mail-Followup-To: autoconf@gnu.org

# Update files maintained in gnulib and autom4te.
.PHONY: fetch
fetch:
	$(PERL) $(srcdir)/build-aux/fetch.pl "$(abs_top_srcdir)"

# Tests not to run.
local-checks-to-skip ?= \
  changelog-check			\
  sc_GPL_version			\
  sc_cast_of_alloca_return_value	\
  sc_m4_quote_check			\
  sc_makefile_at_at_check		\
  sc_prohibit_HAVE_MBRTOWC		\
  sc_prohibit_always-defined_macros	\
  sc_prohibit_always_true_header_tests	\
  sc_prohibit_magic_number_exit		\
  sc_prohibit_stat_st_blocks		\
  sc_unmarked_diagnostics		\
  sc_unportable_grep_q

# Always use shorthand copyrights.
update-copyright-env = \
  UPDATE_COPYRIGHT_USE_INTERVALS=1 \
  UPDATE_COPYRIGHT_MAX_LINE_LENGTH=72

update-copyright: update-release-year
update-release-year:
	$(AM_V_GEN):; \
	if test -n "$$UPDATE_COPYRIGHT_YEAR"; then \
	   current_year=$$UPDATE_COPYRIGHT_YEAR; \
	else \
	  current_year=`date +%Y` && test -n "$$current_year" \
	    || { echo "$@: cannot get current year" >&2; exit 1; }; \
	fi; \
	sed -i "/^RELEASE_YEAR=/s/=.*$$/=$$current_year/" configure.ac
.PHONY: update-release-year

# Prevent incorrect NEWS edits.
old_NEWS_hash = 18aef204e16f6fe9487ab79ca2556ab4

# Update autoconf-latest.tar.* symlinks during 'make stable/beta'.
GNUPLOADFLAGS = --symlink-regex

exclude_file_name_regexp--sc_prohibit_undesirable_word_seq = \
  ^(maint\.mk|build-aux/texinfo\.tex)$$
exclude_file_name_regexp--sc_prohibit_test_minus_ao = \
  ^(maint\.mk|doc/autoconf\.texi)$$
exclude_file_name_regexp--sc_prohibit_atoi_atof = ^doc/autoconf\.texi$$
exclude_file_name_regexp--sc_useless_cpp_parens = \
  ^(build-aux/config\.guess|doc/standards\.texi)$$
exclude_file_name_regexp--sc_trailing_blank = ^build-aux/texinfo\.tex$$
exclude_file_name_regexp--sc_two_space_separator_in_usage = \
  ^build-aux/gnupload$$
exclude_file_name_regexp--sc_prohibit_defined_have_decl_tests = \
  ^doc/autoconf\.texi$$
