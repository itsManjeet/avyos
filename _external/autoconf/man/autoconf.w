#! /bin/sh
# help-extract wrapper, see build-aux/help-extract.pl and man/local.mk
exec "${PERL-perl}" "${top_srcdir}/build-aux/help-extract.pl" \
   bin/autoconf.in "$@"
