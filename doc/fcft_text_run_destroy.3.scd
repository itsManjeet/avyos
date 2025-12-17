fcft_text_run_destroy(3) "3.3.2" "fcft"

# NAME

fcft_text_run_destroy - free fcft_text_run objects

# SYNOPSIS

*\#include <fcft/fcft.h>*

*void fcft_text_run_destroy(struct fcft_text_run \**_run_*);*

# DESCRIPTION

*fcft_text_run_destroy*() frees the *fcft_text_run* object _run_,
which must have been created with *fcft_text_run_rasterize*().

Note that it is ok to call *fcft_destroy*() on the font object that
was used to rasterize the text-run, before freeing the text-run
itself. I.e. applications may cache text-runs beyond the life time of
the font object.

_run_ may be NULL, in which case *fcft_text_run_destroy*() is a no-op.

# SEE ALSO

*fcft_text_run_rasterize*()
