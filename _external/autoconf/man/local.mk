# Make Autoconf man pages.

# Copyright (C) 2001, 2004-2017, 2020-2023 Free Software Foundation,
# Inc.

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

dist_man_MANS = \
  man/autoconf.1 \
  man/autoheader.1 \
  man/autom4te.1 \
  man/autoreconf.1 \
  man/autoscan.1 \
  man/autoupdate.1 \
  man/ifnames.1

EXTRA_DIST += $(dist_man_MANS:.1=.w) $(dist_man_MANS:.1=.x) man/common.x

# Each manpage depends on:
# - its .w and .x files and its source script in bin/
# - common.x for the SEE ALSO list
# - lib/Autom4te/ChannelDefs.pm which contains additional --help text
#   (not included in _all_ the manpages, but it's easier to have them
#   all depend on it)
# - .version and configure.ac for version information
#
# We ship the manpages in tarball releases so people can build from
# them without having help2man installed.  For this to work correctly,
# the manpages cannot have declared dependencies on any file that is
# not also shipped in the tarball.  To avoid concurrency bugs, those
# files plus the Makefile must in fact be sufficient to generate the
# manpages.  See the automake manual, section 'Errors with distclean',
# for further discussion.

binsrcdir      = $(top_srcdir)/bin
channeldefs_pm = lib/Autom4te/ChannelDefs.pm
man_common_dep = $(top_srcdir)/man/common.x \
		 $(top_srcdir)/$(channeldefs_pm) \
		 $(top_srcdir)/.version \
		 $(top_srcdir)/configure.ac

man/autoconf.1:   $(common_dep) man/autoconf.w   man/autoconf.x   $(binsrcdir)/autoconf.in
man/autoheader.1: $(common_dep) man/autoheader.w man/autoheader.x $(binsrcdir)/autoheader.in
man/autom4te.1:   $(common_dep) man/autom4te.w   man/autom4te.x   $(binsrcdir)/autom4te.in
man/autoreconf.1: $(common_dep) man/autoreconf.w man/autoreconf.x $(binsrcdir)/autoreconf.in
man/autoscan.1:   $(common_dep) man/autoscan.w   man/autoscan.x   $(binsrcdir)/autoscan.in
man/autoupdate.1: $(common_dep) man/autoupdate.w man/autoupdate.x $(binsrcdir)/autoupdate.in
man/ifnames.1:    $(common_dep) man/ifnames.w    man/ifnames.x    $(binsrcdir)/ifnames.in

# To generate the manpages, we use help2man, but we don't have it run
# the built script that corresponds to the manpage.  Instead it runs
# the .w file listed above, which is a wrapper around
# build-aux/help-extract.pl, which parses the *source* of the script
# and extracts the help and version text.  This means that the built
# script doesn't need to exist to create the manpage.  If it did,
# we would have a concurrency bug, since we can't declare a dependency
# on the built script, as discussed above.
# We use a suffix rule describing the manpage as built from its .w file
# so that we can use $(<F) to name the executable to be run, which avoids
# an extra subshell and sed invocation.

remove_time_stamp = 's/^\(\.TH[^"]*"[^"]*"[^"]*\)"[^"]*"/\1/'
SUFFIXES += .w .1

.w.1:
	@echo "Updating man page $@"
	$(MKDIR_P) $(@D)
	PATH="$(top_srcdir)/man$(PATH_SEPARATOR)$$PATH"; \
	PERL="$(PERL)"; \
	PACKAGE_NAME="$(PACKAGE_NAME)"; \
	VERSION="$(VERSION)"; \
	RELEASE_YEAR="$(RELEASE_YEAR)"; \
	top_srcdir="$(top_srcdir)"; \
	channeldefs_pm="$(channeldefs_pm)"; \
	export PATH PERL PACKAGE_NAME VERSION RELEASE_YEAR; \
	export top_srcdir channeldefs_pm; \
	$(HELP2MAN) \
	    --include=$(srcdir)/$*.x \
	    --include=$(srcdir)/man/common.x \
	    --source='$(PACKAGE_STRING)' \
	    --output=$@.t $(<F)
	if $(SED) $(remove_time_stamp) $@ >$@a.t 2>/dev/null && \
	   $(SED) $(remove_time_stamp) $@.t | cmp $@a.t - >/dev/null 2>&1; then \
		touch $@; \
	else \
		mv $@.t $@; \
	fi
	rm -f $@.t $@a.t


MOSTLYCLEANFILES     += $(dist_man_MANS:=.t) $(dist_man_MANS:=a.t)
MAINTAINERCLEANFILES += $(dist_man_MANS)

# To satisfy 'distcleancheck', we need to delete built manpages in
# 'distclean' when the build and source directories are not the same.
# We know we are in this case when 'man/common.x' doesn't exist.
distclean-local: distclean-local-man
distclean-local-man:
	test -f man/common.x || rm -f $(dist_man_MANS)
