noinst_HEADERS=fcconst.h

fcconst.h: fc-const.py fcconst.list $(srcdir)/../src/fcobjs.h Makefile.am
	$(AM_V_GEN) $(PYTHON) $(srcdir)/fc-const.py $(srcdir)/fcconst.list $(srcdir)/../src/fcobjs.h > stamp-h && mv stamp-h $@

CLEANFILES = stamp-h
MAINTAINERCLEANFILES = fcconst.h
