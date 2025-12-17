# Copyright (C) 2025 fontconfig Authors
# SPDX-License-Identifier: HPND

bin_PROGRAMS=fc-genconf

DOC2MAN = docbook2man

FC_GENCONF_SRC=${top_srcdir}/fc-genconf

SGML = ${FC_GENCONF_SRC}/fc-genconf.sgml

AM_CPPFLAGS=-I${top_srcdir} $(WARN_CFLAGS)

BUILT_MANS=fc-genconf.1

if ENABLE_DOCS
man_MANS=${BUILT_MANS}
endif

EXTRA_DIST=fc-genconf.sgml $(BUILT_MANS)

CLEANFILES =

fc_genconf_LDADD = ${top_builddir}/src/libfontconfig.la

if USEDOCBOOK

${man_MANS}: ${SGML}
	$(AM_V_GEN) $(RM) $@; \
	$(DOC2MAN) ${SGML}; \
	$(RM) manpage.*

all-local: $(man_MANS)

CLEANFILES += ${man_MANS}
else
all-local:
endif

-include $(top_srcdir)/git.mk
