fcft_font_options_destroy(3) "3.3.2" "fcft"

# NAME

fcft_font_options_destroy - free fcft_font_options objects

# SYNOPSIS

*\#include <fcft/fcft.h>*

*void fcft_font_options_destroy(struct fcft_font_options \**_options_*);*

# DESCRIPTION

*fcft_font_options_destroy*() frees the *fcft_font_options* object
_options_, which must have been created with
*fcft_font_options_create()*.

_options_ may be NULL, in which case *fcft_font_options_destroy*() is
a no-op.

# SEE ALSO

*fcft_font_options_create*()
