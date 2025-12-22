## Make Autoconf tests.

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

# We don't actually distribute the built testsuite or package.m4, since one
# only needs m4 to build them, and m4 is required to install Autoconf.
# But if you are borrowing from this file for setting up autotest in your
# project, remember to distribute both testsuite and package.m4.
EXTRA_DIST += \
  tests/local.at \
  tests/mktests.pl \
  tests/atlocal.in \
  tests/wrapper.as \
  tests/statesave.m4

# Running the uninstalled scripts.  Build them upon 'all', for the manpages.
noinst_SCRIPTS = $(wrappers)
DISTCLEANFILES += tests/atconfig tests/atlocal $(TESTSUITE)

# The ':;' works around a redirected compound command bash exit status bug.
tests/package.m4: Makefile
	:;{ \
	  echo '# Signature of the current package.' && \
	  echo 'm4_define([AT_PACKAGE_NAME],      [$(PACKAGE_NAME)])' && \
	  echo 'm4_define([AT_PACKAGE_TARNAME],   [$(PACKAGE_TARNAME)])' && \
	  echo 'm4_define([AT_PACKAGE_VERSION],   [$(PACKAGE_VERSION)])' && \
	  echo 'm4_define([AT_PACKAGE_STRING],    [$(PACKAGE_STRING)])' && \
	  echo 'm4_define([AT_PACKAGE_BUGREPORT], [$(PACKAGE_BUGREPORT)])' && \
	  echo 'm4_define([AT_PACKAGE_URL],       [$(PACKAGE_URL)])'; \
	} > $@-t
	mv $@-t $@



## ---------- ##
## Wrappers.  ##
## ---------- ##

wrappers = \
  tests/autoconf \
  tests/autoheader \
  tests/autom4te \
  tests/autoreconf \
  tests/autoscan \
  tests/autoupdate \
  tests/ifnames

CLEANFILES += \
  tests/package.m4 \
  tests/wrapper.in \
  $(wrappers)

tests/wrapper.in: $(srcdir)/tests/wrapper.as $(m4sh_m4f_dependencies)
	$(MY_AUTOM4TE) --language=M4sh $(srcdir)/tests/wrapper.as -o $@

edit_wrapper = sed \
	-e 's|@wrap_program[@]|$(@F)|g' \
	-e 's|@abs_top_srcdir[@]|$(abs_top_srcdir)|g' \
	-e 's|@abs_top_builddir[@]|$(abs_top_builddir)|g' \
	-e "s|@configure_input[@]|Generated from $$input.|g"

$(wrappers): tests/wrapper.in
	rm -f $@ $@.tmp
	input=tests/wrapper.in \
	  && $(edit_wrapper) tests/wrapper.in >$@.tmp
	chmod +x $@.tmp
	chmod a-w $@.tmp
	mv -f $@.tmp $@



## ------------ ##
## Test suite.  ##
## ------------ ##

TESTSUITE_GENERATED_AT = \
  tests/aclang.at \
  tests/acc.at \
  tests/acerlang.at \
  tests/acfortran.at \
  tests/acgo.at \
  tests/acgeneral.at \
  tests/acstatus.at \
  tests/acautoheader.at \
  tests/acautoupdate.at \
  tests/acspecific.at \
  tests/acfunctions.at \
  tests/acheaders.at \
  tests/actypes.at \
  tests/aclibs.at \
  tests/acprograms.at

TESTSUITE_HAND_AT = \
  tests/suite.at \
  tests/m4sugar.at \
  tests/m4sh.at \
  tests/autotest.at \
  tests/base.at \
  tests/tools.at \
  tests/torture.at \
  tests/compile.at \
  tests/c.at \
  tests/erlang.at \
  tests/fortran.at \
  tests/go.at \
  tests/semantics.at \
  tests/autoscan.at \
  tests/foreign.at

TESTSUITE_EXTRA = \
  tests/data/ax_prog_cc_for_build_v18.m4 \
  tests/data/ax_prog_cxx_for_build_v3.m4 \
  tests/data/gnulib_std_gnu11_2020_08_17.m4

CLEANFILES += $(TESTSUITE_GENERATED_AT)
EXTRA_DIST += $(TESTSUITE_HAND_AT) $(TESTSUITE_EXTRA)

TESTSUITE_AT = $(TESTSUITE_GENERATED_AT) $(TESTSUITE_HAND_AT)
TESTSUITE = tests/testsuite

# Run the non installed autom4te.
# Don't use AUTOM4TE since 'make alpha' makes it unavailable although
# we are allowed to use it (since we ship it).
AUTOTESTFLAGS = -I tests -I $(srcdir)/tests
AUTOTEST = $(MY_AUTOM4TE) --language=autotest
$(TESTSUITE): tests/package.m4 \
	      tests/local.at \
	      $(TESTSUITE_AT) \
	      lib/autotest/autotest.m4f
	$(AUTOTEST) $(AUTOTESTFLAGS) suite.at -o $@.tmp
	mv $@.tmp $@

# Factor out invocation of the testsuite script.
run_testsuite = $(SHELL) $(TESTSUITE) -C tests MAKE=$(MAKE)

# Avoid a race condition that would make parallel "distclean" fail.
# The rule in clean-local tests for existence of $(TESTSUITE), and
# if found, attempts to run it.  But the distclean-generic rule may
# be running in parallel, and it removes $(DISTCLEANFILES) which
# includes $(TESTSUITE).  This is the Automake rule, plus our
# dependency, and we silence the warning from 'automake -Wall' by
# hiding the dependency behind a variable.
# TODO - fix this if newer automake accommodates the dependency.
distclean_generic = distclean-generic
$(distclean_generic): clean-local

clean-local:
	test ! -f $(TESTSUITE) || $(run_testsuite)  --clean
	rm -f *.tmp
	rm -f -r autom4te.cache

check-local: tests/atconfig tests/atlocal $(TESTSUITE)
	+$(run_testsuite) $(TESTSUITEFLAGS)

# Automake doesn't know how to regenerate this file because
# it's created via AC_CONFIG_COMMANDS.
tests/atconfig: $(top_builddir)/config.status
	cd $(top_builddir) && $(SHELL) ./config.status $@

# Run the test suite on the *installed* tree.
installcheck-local: tests/atconfig tests/atlocal $(TESTSUITE)
	+$(run_testsuite) AUTOTEST_PATH="$(bindir)" $(TESTSUITEFLAGS)



## ------------------ ##
## Maintainer rules.  ##
## ------------------ ##

MAINTAINERCLEANFILES += $(TESTSUITE_GENERATED_AT)

## Producing the test files.

# The files which contain macros we check for syntax.  Use $(srcdir)
# for the benefit of non-GNU make.
autoconfdir = $(srcdir)/lib/autoconf
AUTOCONF_FILES = $(autoconfdir)/general.m4 \
		 $(autoconfdir)/status.m4 \
		 $(autoconfdir)/autoheader.m4 \
		 $(autoconfdir)/autoupdate.m4 \
		 $(autoconfdir)/specific.m4 \
		 $(autoconfdir)/functions.m4 \
		 $(autoconfdir)/lang.m4 \
		 $(autoconfdir)/c.m4 \
		 $(autoconfdir)/erlang.m4 \
		 $(autoconfdir)/fortran.m4 \
		 $(autoconfdir)/go.m4 \
		 $(autoconfdir)/headers.m4 \
		 $(autoconfdir)/libs.m4 \
		 $(autoconfdir)/types.m4 \
		 $(autoconfdir)/programs.m4

$(TESTSUITE_GENERATED_AT): tests/mktests.stamp
## Recover from the removal of $@
	@if test -f $@; then :; else \
	  rm -f tests/mktests.stamp; \
	  $(MAKE) $(AM_MAKEFLAGS) tests/mktests.stamp; \
	fi

tests/mktests.stamp : tests/mktests.pl $(AUTOCONF_FILES)
	@rm -f tests/mktests.tmp
	@touch tests/mktests.tmp
	$(PERL) $(srcdir)/tests/mktests.pl tests $(AUTOCONF_FILES)
	@mv -f tests/mktests.tmp $@

CLEANFILES += tests/mktests.tmp tests/mktests.stamp

## maintainer-check ##

# The test suite cannot be run in parallel with itself.
maintainer-check:
	$(MAKE) $(AM_MAKEFLAGS) check
	$(MAKE) $(AM_MAKEFLAGS) maintainer-check-posix

# The hairy heredoc is more robust than using echo.
CLEANFILES += expr
expr:
	:;{					\
	  echo '#! $(SHELL)';			\
	  echo 'result=`$(EXPR) "$$@"`';	\
	  echo 'estatus=$$?';			\
	  echo 'cat <<EOF';			\
	  echo '$${result:-0}';			\
	  echo 'EOF';				\
	  echo 'exit $$estatus';		\
	} > $@-t
	chmod +x $@-t
	mv $@-t $@

# Try the test suite with more severe environments.
maintainer-check-posix: expr
	POSIXLY_CORRECT=yes $(MAKE) $(AM_MAKEFLAGS) check
	rm expr
