## -*- makefile-automake -*-
## Copyright (C) 1995-2025 Free Software Foundation, Inc.
##
## This program is free software; you can redistribute it and/or modify
## it under the terms of the GNU General Public License as published by
## the Free Software Foundation; either version 2, or (at your option)
## any later version.
##
## This program is distributed in the hope that it will be useful,
## but WITHOUT ANY WARRANTY; without even the implied warranty of
## MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
## GNU General Public License for more details.
##
## You should have received a copy of the GNU General Public License
## along with this program.  If not, see <https://www.gnu.org/licenses/>.

## ---------------- ##
##  Documentation.  ##
## ---------------- ##

info_TEXINFOS = %D%/automake.texi %D%/automake-history.texi
doc_automake_TEXINFOS = %D%/fdl.texi
doc_automake_history_TEXINFOS = %D%/fdl.texi

man1_MANS = \
  %D%/aclocal.1 \
  %D%/automake.1 \
  %D%/aclocal-$(APIVERSION).1 \
  %D%/automake-$(APIVERSION).1

$(man1_MANS): $(top_srcdir)/configure.ac
CLEANFILES += $(man1_MANS)

# In automake, users generate man pages as part of a normal build from
# release tarballs. This is ok because we also distribute the help2man
# script, as given below.
#
# Autoconf handles this in an alternative way, of including the man
# pages in the tarballs and thus not requiring help2man to be run by
# users (q.v.). Neither is better or worse than the other.
#
# See the "Errors with distclean" node in the manual for more info.

# XXX: The help2man script we include in the Automake distribution
# should be updated with 'fetch' target, but isn't. Instead, you must
# build help2man normally and copy it in manually. Keep the first line as:
#   #!/usr/bin/perl -w
# whatever it might have ended up as on your system.
EXTRA_DIST += %D%/help2man

update_mans = \
    $(MKDIR_P) %D% \
    && AUTOMAKE_HELP2MAN=true ./pre-inst-env \
       $(PERL) $(srcdir)/%D%/help2man --output=$@ --info-page=automake \
               --name="$${HELP2MAN_NAME}"

%D%/aclocal.1 %D%/automake.1:
	$(AM_V_GEN): \
	  && $(MKDIR_P) %D% \
	  && f=`echo $@ | sed 's|.*/||; s|\.1$$||; $(transform)'` \
	  && echo ".so man1/$$f-$(APIVERSION).1" > $@

%D%/aclocal-$(APIVERSION).1: $(aclocal_script) lib/Automake/Config.pm
	$(AM_V_GEN):; HELP2MAN_NAME="Generate aclocal.m4 by scanning configure.ac"; export HELP2MAN_NAME; $(update_mans) $(aclocal_script)
%D%/automake-$(APIVERSION).1: $(automake_script) lib/Automake/Config.pm
	$(AM_V_GEN):; HELP2MAN_NAME="Generate Makefile.in files for configure from Makefile.am"; export HELP2MAN_NAME; $(update_mans) $(automake_script)

## This checklinkx target is not invoked as a dependency of anything.
## It exists merely to make checking the links in automake.texi (that is,
## automake.html) more convenient. We use a slightly-enhanced version of
## W3C checklink to do this. We intentionally do not have automake.html
## as a dependency, as it seems more convenient to have its regeneration
## under manual control. See https://debbugs.gnu.org/10371.
##
checklinkx = $(top_srcdir)/contrib/checklinkx
# that particular sleep seems to be what gnu.org likes.
chlx_args = -v --sleep 8 #--exclude-url-file=/tmp/xf
# Explanation of excludes:
# - w3.org dtds, they are fine (and slow).
# - mailto urls, they are always forbidden.
# - vala, redirects to a Gnome subpage and returns 403 to us.
# - cfortran, forbidden by site's robots.txt.
# - debbugs.gnu.org/automake, forbidden by robots.txt.
# - autoconf.html, forbidden by robots.txt (since served from savannah).
# - https://fsf.org redirects to https://www.fsf.org and nothing to do
#   (it's in the FDL).  --suppress-redirect options do not suppress the msg.
#
chlx_excludes = \
    -X 'http.*w3\.org/.*dtd' \
    -X 'mailto:.*' \
    -X 'https://www\.vala-project\.org/' \
    -X 'https://www-zeus\.desy\.de/~burow/cfortran/' \
    -X 'https://debbugs\.gnu\.org/automake' \
    -X 'https://www\.gnu\.org/software/autoconf/manual/autoconf\.html' \
    -X 'https://fsf\.org/'
chlx_file = $(top_srcdir)/doc/automake.html
.PHONY: checklinkx
checklinkx:
	$(checklinkx) $(chlx_args) $(chlx_excludes) $(chlx_file)

## ---------------------------- ##
##  Example package "amhello".  ##
## ---------------------------- ##

amhello_sources = \
  %D%/amhello/configure.ac \
  %D%/amhello/Makefile.am \
  %D%/amhello/README \
  %D%/amhello/src/main.c \
  %D%/amhello/src/Makefile.am

amhello_configury = \
  aclocal.m4 \
  autom4te.cache \
  Makefile.in \
  config.h.in \
  configure \
  depcomp \
  install-sh \
  missing \
  src/Makefile.in

dist_noinst_DATA += $(amhello_sources)
dist_doc_DATA = $(srcdir)/%D%/amhello-1.0.tar.gz

setup_autotools_paths = { \
  ACLOCAL=aclocal-$(APIVERSION) && export ACLOCAL \
    && AUTOMAKE=automake-$(APIVERSION) && export AUTOMAKE \
    && AUTOCONF='$(am_AUTOCONF)' && export AUTOCONF \
    && AUTOM4TE='$(am_AUTOM4TE)' && export AUTOM4TE \
    && AUTORECONF='$(am_AUTORECONF)' && export AUTORECONF \
    && AUTOHEADER='$(am_AUTOHEADER)' && export AUTOHEADER \
    && AUTOUPDATE='$(am_AUTOUPDATE)' && export AUTOUPDATE \
    && true; \
}

# We depend on configure.ac so that we regenerate the tarball
# whenever the Automake version changes.
$(srcdir)/%D%/amhello-1.0.tar.gz: $(amhello_sources) $(srcdir)/configure.ac
	$(AM_V_GEN)tmp=amhello-output.tmp \
	  && $(am__cd) $(srcdir)/%D%/amhello \
	  && : Make our	aclocal and automake available before system ones. \
	  && $(setup_autotools_paths) \
	  && ( \
	    { $(AM_V_P) || exec 5>&2 >$$tmp 2>&1; } \
	      && $(abs_builddir)/pre-inst-env $(am_AUTORECONF) -vfi \
	      && ./configure \
	      && $(MAKE) $(AM_MAKEFLAGS) distcheck \
	      && $(MAKE) $(AM_MAKEFLAGS) distclean \
	      || { \
	        if $(AM_V_P); then :; else \
	          echo "$@: recipe failed." >&5; \
	          echo "See file '`pwd`/$$tmp' for details" >&5; \
		fi; \
	        exit 1; \
	      } \
	  ) \
	  && rm -rf $(amhello_configury) $$tmp \
	  && mv -f amhello-1.0.tar.gz ..


# vim: ft=automake noet
