fcft_rasterize_text_run_utf32(3) "3.3.2" "fcft"

# NAME

fcft_rasterize_text_run_utf32 - rasterize a series of glyphs for a text string

# SYNOPSIS

*\#include <fcft/fcft.h>*

*struct fcft_text_run \*fcft_rasterize_text_run_utf32(*
	*struct fcft_font \**_font_*, size_t *_len_*,*
	*const uint32_t *_text_*[static len], enum fcft_subpixel *_subpixel_*);*

# DESCRIPTION

*fcft_rasterize_text_run_utf32*() shapes and rasterizes the UTF-32
encoded Unicode string _text_ into a series of glyphs using the
primary font, and possibly one or more fallback fonts, in _font_.

_subpixel_ allows you to specify which subpixel mode to use. See
*fcft_rasterize_char_utf32*() for details.

The string is first segmented into grapheme clusters using
utf8proc. Each grapheme is assigned a font using the normal font
lookup rules (see *fcft_rasterize_char_utf32*()).

Next, HarfBuzz is used to guess each grapheme's script
(~language). Consecutiv graphemes that both have been assigned the
same font, and belong to the same script are merged into a "partial"
text run.

Finally, each partial text run is shaped with HarfBuzz.

# RETURN VALUE

On error, NULL is returned.

On success, a pointer to a dynamically allocated text-run is returned:

```
struct fcft_text_run {
    const struct fcft_glyph **glyphs;
    int *cluster;
    size_t count;
};
```

Both _glyphs_ and _cluster_ are arrays with _count_
elements. _cluster_ contains the character offsets (in the original
string) of each corresponding glyph.

Note that the glyphs' _cols_ member are invalid.

The text-run is not cached in fcft. The calling application may cache
it for as long as it likes, including after the font has been
destroyed.

The text-run must be free:d with *fcft_text_run_destroy*().

# SEE ALSO

*fcft_text_run_destroy*(), *fcft_rasterize_char_utf32*(),
*fcft_rasterize_grapheme_utf32*()
