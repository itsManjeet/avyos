fcft_rasterize_char_utf32(3) "3.3.2" "fcft"

# NAME

fcft_rasterize_char_utf32 - rasterize a glyph for a single UTF-32 codepoint

# SYNOPSIS

*\#include <fcft/fcft.h>*

*const struct fcft_glyph \*fcft_rasterize_char_utf32(*
	*struct fcft_font \**_font_*, uint32_t *_cp_*,
	enum fcft_subpixel *_subpixel_*);*

# DESCRIPTION

*fcft_rasterize_char_utf32*() rasterizes the UTF-32 encoded Unicode
codepoint _cp_ using the primary font, or one of the fallback fonts,
in _font_.

_cp_ is first searched for in the primary font. If not found, the
fallback fonts are searched (in the order they were specified in
*fcft_from_name*()). If not found in any of the fallback fonts, the
FontConfig fallback list for the primary font is searched.

_subpixel_ allows you to specify which subpixel mode to use. It is one of:

```
enum fcft_subpixel {
    FCFT_SUBPIXEL_DEFAULT,
    FCFT_SUBPIXEL_NONE,
    FCFT_SUBPIXEL_HORIZONTAL_RGB,
    FCFT_SUBPIXEL_HORIZONTAL_BGR,
    FCFT_SUBPIXEL_VERTICAL_RGB,
    FCFT_SUBPIXEL_VERTICAL_BGR,
};
```

If *FCFT\_SUBPIXEL\_DEFAULT* is specified, the subpixel mode
configured in FontConfig is used. If *FCFT\_SUBPIXEL\_NONE* is
specified, grayscale antialiasing will be used. For all other values,
the specified mode is used.

Note that if antialiasing has been disabled (in FontConfig, either
globally, or specifically for the current font), then _subpixel_ is
ignored.

The intention is to enable programs to use per-monitor subpixel
modes. Incidentally, *enum fcft_subpixel* matches *enum
wl_output_subpixel*, the enum used in Wayland.

Note: you probably do not want to use anything other than
*FCFT_SUBPIXEL_NONE* if blending with a transparent background.

# RETURN VALUE

On error, NULL is returned.

On success, a pointer to a rasterized glyph is returned. The glyph is
cached in fcft, making subsequent calls with the same arguments very
fast (i.e. there is no need for programs to cache glyphs by
themselves).

The glyph object is managed by _font_. There is no need to explicitly
free it; it is freed when _font_ is destroyed (with
*fcft_destroy*()).

```
struct fcft_glyph {
    uint32_t cp;
    int cols;

    const char *font_name;
    pixman_image_t *pix;

    int x;
    int y;
    int width;
    int height;

    struct {
        int x;
        int y;
    } advance;

    bool is_color_glyph;
};
```

_cp_ is the same _cp_ from the *fcft_rasterize_char_utf32*() call.

_cols_ is the number of "columns" the glyph occupies (effectively,
*wcwidth*(_cp_*)*). Note that this value will be incorrect if wide
characters (*wchar_t*) is not UTF-32 encoded in the current locale.

_pix_ is the rasterized glyph. Its format depends on a number of
factors, but will be one of *PIXMAN\_a1*, *PIXMAN\_a8*,
*PIXMAN\_x8r8g8b8*, *PIXMAN\_a8r8g8b8*. Use
*pixman_image_get_format*() to find out which one it is.

	*PIXMAN\_a1* corresponds to *FT\_PIXEL\_MODE\_MONO*. I.e. the
	glyph is an un-antialiased bitmask. Use as a mask when blending.

	*PIXMAN\_a8* corresponds to *FT\_PIXEL\_MODE\_GRAY*. I.e. the
	glyph is a grayscale antialiased bitmask. Use as a mask when
	blending.

	*PIXMAN\_x8r8g8b8* corresponds to either *FT\_PIXEL\_MODE\_LCD* or
	*FT\_PIXEL\_MODE\_LCD\_V*. *pixman_image_set_component_alpha*()
	has been called by fcft for you. Use as a mask when blending.

	*PIXMAN\_a8r8g8b8* corresponds to
	*FT\_PIXEL\_MODE\_BGRA*. I.e. the glyph is a plain RGBA image. Use
	as source when blending.

_x_ is the glyph's horizontal offset, in pixels. Add this to the
current pen position when blending.

_y_ is the glyph's vertical offset, in pixels. Add this to the current
pen position when blending.

_width_ is the glyph's width, in pixels. Use as 'width' argument when
blending.

_height_ is the glyph's height, in pixels. Use as 'height' argument
when blending.

_advance_ is the glyph's 'advance', in pixels. Add this to the pen
position after blending; _x_ for a horizontal layout and _y_ for a
vertical layout.

_is_color_glyph_ is true if the glyph is a color glyph, typically an
emoji. When true, the glyph should be composited OVER the destination
surface, instead of being used as a mask.

# EXAMPLE

See *fcft_from_name*()

# SEE ALSO

*fcft_destroy*(), *fcft_kerning*(), *fcft_rasterize_grapheme_utf32*(),
*fcft_rasterize_text_run_utf32*()
