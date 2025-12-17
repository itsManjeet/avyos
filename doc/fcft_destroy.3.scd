fcft_destroy(3) "3.3.2" "fcft"

# NAME

fcft_destroy - free fcft_font objects

# SYNOPSIS

*\#include <fcft/fcft.h>*

*void fcft_destroy(struct fcft_font \**_font_*);*

# DESCRIPTION

*fcft_destroy*() frees the *fcft_font* object _font_, which must have
been created with *fcft_from_name*() or *fcft_clone*().

After calling *fcft_destroy*(), _font_ can no longer be used, and all
references to *fcft_glyph* objects (retrieved with
*fcft_codepoint_rasterize*()) are now invalid.

_font_ may be NULL, in which case *fcft_destroy*() is a no-op.

# RETURN VALUE

None

# SEE ALSO

*fcft_from_name*(), *fcft_from_name2*(), *fcft_clone*(),
*fcft_codepoint_rasterize*()
