fcft_fini(3) "3.3.2" "fcft"

# NAME

fcft_fini - release global resources

# SYNOPSIS

*\#include <fcft/fcft.h>*

*void fcft_fini(*void*);*

# DESCRIPTION

Free's all global resources: the font cache, thread synchronization
primitives, and calls *FT_Done_FreeType*() and *FcFini*().

Calling *fcft_fini*() is usually not required, but is needed for clean
Valgrind runs.

# SEE ALSO

*fcft_init*(), *FT_Done_FreeType*(), *FcFini*()
