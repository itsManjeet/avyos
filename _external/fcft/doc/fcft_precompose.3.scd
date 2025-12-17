fcft_precompose(3) "3.3.2" "fcft"

# NAME

fcft_precompose - pre-compose two wide characters into one

# SYNOPSIS

*\#include <fcft/fcft.h>*

*bool fcft_precompose(*
	*const struct fcft_font \**_font_*,*
	*uint32_t *_base_*, uint32_t *_comb_*,*
	*bool \**_base_is_from_primary_*,*
	*bool \**_comb_is_from_primary_*,*
	*bool \**_composed_is_from_primary_*);*

# DESCRIPTION

*fcft_precompose*() composes the _base_ and _comb_ wide characters
into a single wide character. For example, *L'a'* + *L'\\u0301'* can
be combined to *L'á'*.

If _\*\_is\_from\_primary_ are non-NULL, they will be *true* if the
corresponding wide character exists in the primary font. If *false*,
it does not exist in the primary font and *fcft_codepoint_rasterize*()
will pick the glyph from one of the fallback fonts.

# RETURN VALUE

On success, *fcft_precompose*() returns a composed wide character. On
error, *(uint32_t)-1* is returned.

# SEE ALSO

*fcft_codepoint_rasterize*(), *fcft_kerning*()
