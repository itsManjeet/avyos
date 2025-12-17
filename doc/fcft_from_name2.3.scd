fcft_from_name(3) "3.3.2" "fcft"

# NAME

fcft_from_name - instantiate a new font

# SYNOPSIS

*\#include <fcft/fcft.h>*

*struct fcft_font \*fcft_from_name2(
	size_t* _count_*, const char \**_names_*[static* _count_*],
	const char \**_attributes_*, const struct fcft_font_options \**_options_*);*

# DESCRIPTION

*fcft_from_name2*() instantiates a new fcft font object from the
FontConfig formatted font names. The first element in _names_ is the
primary font, and the remaining elements (if any) are fallback fonts.

You *must* supply at least one font name.

All aspects of the font (size, DPI, variant etc) are configured
through the font _name_, using colon separated *attribute=value* pairs
(e.g. *"Serif:size=26:slant=italic"*).

_attributes_ is a convenient way to apply a set of attributes to all
fonts in _names_. _attributes_ may be NULL, in which case no extra
attributes are appended to the strings in _names_.

The font size is set as part of the font name:
e.g. *Helvetica:size=12*, or *Helvetica:pixelsize=12*.

To implement desktop scaling, simply multiply the font's size with the
desktop scaling factor.

The primary font will be instantiated immediately, and any failure to
do so will result in an error. Fallback fonts are instantiated on
demand, and any failure to do so will result in the that fallback font
being ignored, and the next one in the list is tried instead.

Rendering details can be controlled through the _options_
parameter. You *must not* allocate _options_ yourself; use
*fcft_font_options_create*(3). You can also pass *NULL*; it is the
same thing as using the value returned from
*fcft_font_options_create*(3) directly, without modifying it (i.e. you
get the default options).

# RETURN VALUE

On success, *fcft_from_name2*() returns a pointer to an allocated
*fcft_font* object. On error, NULL is returned.

```
struct fcft_font {
    int height;
    int descent;
    int ascent;

    struct {
        int x;
        int y;
    } max_advance;

    struct {
        int x;
        int y;
    } space_advance;

    struct {
        int position;
        int thickness;
    } underline;

    struct {
        int position;
        int thickness;
    } strikeout;
};
```

_height_ is the line height, in pixels.

_descent_ is the distance between the baseline and the font's lowest
descending glyph, in pixels. In fcft's case, it is generally positive
(a negative value means the _descent_ stretches *up* from the
baseline).

_ascent_ is the distance between the baseline and the font's highest
ascending glyph, in pixels. Generally a positive number (a negative
value means the _ascent_ stretches *down* below the baseline).

_ascent_ + _descent_ is often the same as _height_, but not
necessarily. _height_ may be larger, meaning the font intentionally
adds extra (vertical) space between lines. Or it may be smaller, in
which case lines overlap.

_max\_advance_ is the amount, in pixels, the font's widest glyph
advances the pen position; _x_ for a horizontal layout, and _y_ for a
vertical layout.

_space\_advance_ is the amount, in pixels, the glyph for *space*
(0x20) advances the pen position; _x_ for a horizontal layout, and _y_
for a vertical layout.

_underline_ describes how to render underlines. _position_ is the
distance, in pixels, from the baseline. A positive value means *above*
the baseline, and a negative value means *below* the
baseline. _thickness_ is the line's thickness, in pixels.

_strikeout_ describes how to render strikeouts. See _underline_ for a
description of its members.

# EXAMPLE

In this example, we instantiate _Times New Roman_ at a point size of 8
as the primary font.

We also tell it to use _Serif Bold_ (point size 10) as a fallback font
(note that it is usually better to let FontConfig handle fallback to
generic fonts like this).

Furthermore, both fonts will be _Italic_, and will be using DPI=140.

We then proceed to render the string _hello world_. You are assumed to
know how to create and use a pixman image. This example only shows how
one can use fcft to instantiate a font, rasterize glyphs and then
blend them onto a target pixman image.


```
#include <stdlib.h>
#include <uchar.h>
#include <fcft/fcft.h>

int
main(void)
{
    setlocale(LC_CTYPE, "en_US.UTF-8");

    if (!fcft_set_scaling_filter(FCFT_SCALING_FILTER_LANCZOS3))
       return EXIT_FAILURE;

    struct fcft_font_options *options = fcft_font_options_create();

    /* Optionally override default options here */

    struct fcft_font *font = fcft_from_name2(
        2,
        (const char *[]){
            "Times New Roman:size=8",
            "Serif:size=10:weight=bold",
        },
        "slant=italic:dpi=140",
        options);

    fcft_font_options_destroy(options);

    if (font == NULL)
        return EXIT_FAILURE;

    /* Here you need to instantiate a 'target' pixman image, to blend
       with */
    pixman_image_t *canvas = ...;

    /* String to print */
    static const char32_t const hello[] = U"hello world";

    /*
     * Pen position in canvas. The numbers chosen here are more or less
     * random. Note however, that the composite calls below assume 'y'
     * is the font's baseline (and thus the glyphs will be rendered
     * above 'y')
     */
    struct {
        int x;
        int y;
    } pen = {.x = 25, .y = 50};

    /* Glyphs will be rendered in white */
    pixman_image_t *color = pixman_image_create_solid_fill(
        &(struct pixman_color_t){
            .red = 0xffff,
            .green = 0xffff,
            .blue = 0xffff,
            .alpha = 0xffff,
        });

    for (size_t i = 0; i < sizeof(hello) / sizeof(hello[0]) - 1; i++) {
        const struct fcft_glyph *glyph = fcft_codepoint_rasterize(
            font, hello[i], FCFT_SUBPIXEL_DEFAULT);

        if (glyph == NULL)
            continue;

        /* Kerning */
        long x_kern = 0;
        if (i > 0) {
            fcft_kerning(font, hello[i - 1], hello[i], &x_kern, NULL);

        pen.x += x_kern;

        if (glyph->is_color_glyph) {
            /* Glyph is a pre-rendered image; typically a color emoji */
            pixman_image_composite32(
                PIXMAP_OP_OVER, glyph->pix, NULL, canvas, 0, 0, 0, 0,
                pen.x + glyph->x, pen.y + font->ascent - glyph->y,
                glyph->width, glyph->height);
        }

        else {
            /* Glyph is an alpha mask */
            pixman_image_composite32(
                PIXMAN_OP_OVER, color, glyph->pix, canvas, 0, 0, 0, 0,
                pen.x + glyph->x, pen.y + font->ascent - glyph->y,
                glyph->width, glyph->height);
        }

        /* Advance pen position */
        pen.x += glyph->advance.x;
    }

    pixman_image_unref(src);

    fcft_destroy(font);
    return EXIT_SUCCESS;
}
```

# SEE ALSO

*fcft_from_name*(), *fcft_clone*(), *fcft_destroy*(),
*fcft_codepoint_rasterize*(), *fcft_kerning*(),
*fcft_font_options_create*(), *fcft_font_options_destroy*()
