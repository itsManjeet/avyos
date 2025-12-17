fcft_kerning(3) "3.3.2" "fcft"

# NAME

fcft_kerning - calculate kerning distance between two wide characters

# SYNOPSIS

*\#include <fcft/fcft.h>*

*bool fcft_kerning(*
	*struct fcft_font \**_font_*, uint32_t *_left_*, uint32_t *_right_*,*
	*long \*restrict *_x_*, long \*restrict *_y_*);*

# DESCRIPTION

*fcft_kerning*() calculates the kerning distances _x_ (horizontal) and
_y_ (vertical) between the two wide characters _left_ and _right_, in
pixels.

Both characters must exist in the primary font.

*fcft_kerning*() is just a convenience function around
*FT_Get_Kerning*() and as such, it only supports legacy _kern_
tables. In particular, OpenType fonts' _GPOS_ tables are *not*
supported. fcft is not a text shaping library.

# RETURN VALUE

On success, *fcft_kerning*() returns true, and _x_ and _y_ are updated
with the kerning distance, in pixels.

On error, false is returned. This may happen for several reasons:

- The primary font does not have any kerning information.
- One, or both, of the two characters have no corresponding glyph in
  the primary font.
- There is no kerning information for this character combination.

# EXAMPLE

See *fcft_from_name*()
