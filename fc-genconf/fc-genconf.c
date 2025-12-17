/* Copyright (C) 2025 fontconfig Authors */
/* SPDX-License-Identifier: HPND */

#ifdef HAVE_CONFIG_H
#include <config.h>
#else
#ifdef linux
#define HAVE_GETOPT_LONG 1
#endif
#define HAVE_GETOPT 1
#endif

#include <fontconfig/fontconfig.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <locale.h>
#include <sys/stat.h>
#include <fcntl.h>

#ifdef HAVE_UNISTD_H
#include <unistd.h>
#endif

#ifdef ENABLE_NLS
#include <libintl.h>
#define _(x)		(dgettext(GETTEXT_PACKAGE, x))
#else
#define dgettext(d, s)	(s)
#define _(x)		(x)
#endif

#ifndef HAVE_GETOPT
#define HAVE_GETOPT 0
#endif
#ifndef HAVE_GETOPT_LONG
#define HAVE_GETOPT_LONG 0
#endif

#if HAVE_GETOPT_LONG
#undef  _GNU_SOURCE
#define _GNU_SOURCE
#include <getopt.h>
static const struct option longopts[] = {
    { "family", 1, 0, 'p' },
    { "generic", 1, 0, 'g' },
    { "lang", 1, 0, 'l' },
    {"output", 1, 0, 'o'},
    {"version", 0, 0, 'V'},
    {"help", 0, 0, 'h'},
    {NULL,0,0,0},
};
#else
#if HAVE_GETOPT
extern char *optarg;
extern int optind, opterr, optopt;
#endif
#endif

static void
usage (char *program, int error)
{
    FILE *file = error ? stderr : stdout;
#if HAVE_GETOPT_LONG
    fprintf (file, _("usage: %s [-Vh]\n"), program);
    fprintf (file, _("usage: %s [--output FILE] [--family NAME] [--generic NAME] [--lang LANG ...] font-path\n"),
	     program);
#else
    fprintf (file, _("usage: %s [-Vh]\n"), program);
    fprintf (file, _("usage: %s [-f NAME] [-g NAME] [-o FILE] [-l LANG ...] font-path\n"),
	     program);
#endif
    fprintf (file, _("Generate config file\n"));
    fprintf (file, "\n");
#if HAVE_GETOPT_LONG
    fprintf (file, _("  -V, --version        display font config version and exit\n"));
    fprintf (file, _("  -h, --help           display this help and exit\n"));
    fprintf (file, _("  -f, --family=NAME    Use NAME as the family\n"));
    fprintf (file, _("  -g, --generic=NAME   Set NAME as the generic-family\n"));
    fprintf (file, _("  -l, --lang=LANG      Use LANG to make a config the language specific conditional\n"));
    fprintf (file, _("  -o, --output=FILE    Output results into FILE (default: stdout)\n"));
#else
    fprintf (file, _("  -V         (version)       display font config version and exit\n"));
    fprintf (file, _("  -h         (help)          display this help and exit\n"));
    fprintf (file, _("  -f NAME    (family)        Use NAME as the family\n"));
    fprintf (file, _("  -g NAME    (generic)       Set NAME as the generic-family\n"));
    fprintf (file, _("  -l LANG    (lang)          Use LANG to make a config the language specific conditional\n"));
    fprintf (file, _("  -o FILE    (output)        Output results into FILE (default: stdout)\n"));
#endif
    exit (error);
}

int
main (int argc, char **argv)
{
    const FcConstant  *constant;
    FcChar8           *out;
    FcPattern         *pat;
    FcLangSet         *ls;
    FILE              *fpo = stdout;
    int                i;
    size_t             len;
#if HAVE_GETOPT_LONG || HAVE_GETOPT
    int c;

    setlocale (LC_ALL, "");

    pat = FcPatternCreate();
    ls = FcLangSetCreate();

#if HAVE_GETOPT_LONG
    while ((c = getopt_long (argc, argv, "f:g:l:m:o:Vh", longopts, NULL)) != -1)
        #else
        while ((c = getopt (argc, argv, "f:g:l:m:o:Vh")) != -1)
#endif
    {
	switch (c) {
	case 'f':
	    FcPatternAddString(pat, FC_FAMILY, (const FcChar8 *)optarg);
	    break;
	case 'g':
	    if (!(constant = FcNameGetConstantFor ((const FcChar8 *)optarg, FC_GENERIC_FAMILY))) {
		char *p = NULL;
		long  n = strtol (optarg, &p, 10);

                if ((p != NULL && *p != 0) || FcNameGetConstantNameFrom(FC_GENERIC_FAMILY, n) == NULL) {
		    fprintf (stderr, "Invalid generic family: %s\n", optarg);
		    usage (argv[0], 1);
		}
		FcPatternAddInteger (pat, FC_GENERIC_FAMILY, n);
	    } else {
		FcPatternAddInteger (pat, FC_GENERIC_FAMILY, constant->value);
	    }
	    break;
	case 'l':
	    if (!FcLangSetAdd (ls, (const FcChar8 *)optarg)) {
		fprintf (stderr, "Unable to add a lang: %s\n", optarg);
		usage(argv[0], 1);
	    }
	    break;
	case 'o':
	    if ((fpo = fopen (optarg, "wb")) == NULL) {
		fprintf (stderr, "Unable to open: %s\n", optarg);
		usage (argv[0], 1);
	    }
	    break;
	case 'V':
	    fprintf (stderr, "fontconfig version %d.%d.%d\n",
		     FC_MAJOR, FC_MINOR, FC_REVISION);
	    exit (0);
	case 'h':
	    usage (argv[0], 0);
	default:
	    usage (argv[0], 1);
	}
    }
    i = optind;
#else
    i = 1;
#endif

    if (i == argc)
	usage (argv[0], 1);

    FcPatternAddLangSet (pat, FC_LANG, ls);

    out = FcConfigFileGenerate(NULL, pat, (const FcChar8 *)argv[i]);
    if (!out)
	usage (argv[0], 1);

    len = strlen ((const char *) out);
    fwrite (out, sizeof (FcChar8), len, fpo);

    if (fpo != stdout)
	fclose (fpo);

    FcStrFree (out);
    FcLangSetDestroy (ls);
    FcPatternDestroy (pat);

    FcFini ();

    return 0;
}
