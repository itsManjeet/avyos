fcft_rasterize_grapheme_utf32(3) "3.3.2" "fcft"

# NAME

fcft_rasterize_grapheme_utf32 - rasterize glyph(s) for a  grapheme cluster

# SYNOPSIS

*\#include <fcft/fcft.h>*

*const struct fcft_grapheme \*fcft_rasterize_grapheme_utf32(*
	*struct fcft_font \**_font_*,*
	*size_t *_len_*, const uint32\_t *_grapheme\_cluster[static len]_*,*
	*enum fcft_subpixel *_subpixel_*);*

# DESCRIPTION

*fcft_rasterize_grapheme_utf32*() rasterizes the grapheme cluster _cluster_
using the primary font, or one of the fallback fonts, in _font_.

Each cluster element is an UTF-32 encoded Unicode codepoint.

The fonts are searched for _all_ codepoints in the grapheme cluster,
starting with the primary font and then the custom fallback fonts, and
finally the FontConfig provided fallback fonts.

_subpixel_ allows you to specify which subpixel mode to use. See
*fcft_rasterize_char_utf32*() for details.

# RETURN VALUE

On error, NULL is returned.

On success, a pointer to a rasterized grapheme is returned. The
grapheme is cached in fcft, making subsequent calls with the same
arguments very fast (i.e. there is no need for programs to cache
glyphs by themselves).

The grapheme object is managed by _font_. There is no need to
explicitly free it; it is freed when _font_ is destroyed (with
*fcft_destroy*()).

```
struct fcft_grapheme {
    int cols;

    size_t count;
    const struct fcft_glyph **glyphs;
};
```

_cols_ is the number of "columns" the glyph occupies (effectively,
*wcswidth*(_cluster_*)*).

_glyphs_ is an array of _count_ rasterized glyphs. See
*fcft_rasterize_char_utf32*() for a description of *struct
fcft_glyph*.

# SEE ALSO

*fcft_destroy*(), *fcft_rasterize_char_utf32*(),
*fcft_rasterize_text_run_utf32*()
