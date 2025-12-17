fcft_font_options_create(3) "3.3.2" "fcft"

# NAME

fcft_font_options_create - instantiate a new font

# SYNOPSIS

*\#include <fcft/fcft.h>*

*struct fcft_font_options \*fcft_font_options_create(*_void_*);*

# DESCRIPTION

*fcft_font_options_create*() allocates a *struct fcft_font_options*
object to be used with *fcft_from_name2*(3).

The returned object will be initialized with default values. Manually
set struct members to override.

Note: you *must* use *fcft_font_options_create*() to allocate the
_options_ object; do *not* allocate it yourself, neither from the heap
or on the stack. This is to ensure forward compatibility.

```
struct fcft_font_options {
    enum fcft_scaling_filter scaling_filter;
    enum fcft_emoji_presentation emoji_presentation;

    struct {
        bool srgb_decode;
        pixman_format_code_t format;
    } color_glyphs;
};
```

# SCALING FILTER

Configures the filter the use when downscaling bitmap fonts (typically
emoji fonts).

Possible values are:

- *FCFT\_SCALING\_FILTER\_NONE*
- *FCFT\_SCALING\_FILTER\_NEAREST*
- *FCFT\_SCALING\_FILTER\_BILINEAR*
- *FCFT\_SCALING\_FILTER\_IMPULSE*
- *FCFT\_SCALING\_FILTER\_BOX*
- *FCFT\_SCALING\_FILTER\_LINEAR*
- *FCFT\_SCALING\_FILTER\_CUBIC*
- *FCFT\_SCALING\_FILTER\_GAUSSIAN*
- *FCFT\_SCALING\_FILTER\_LANCZOS2*
- *FCFT\_SCALING\_FILTER\_LANCZOS3*
- *FCFT\_SCALING\_FILTER\_LANCZOS3\_STRETCHED*

*FCFT\_SCALING\_FILTER\_NONE* disables filtering.

*FCFT\_SCALING\_FILTER\_NEAREST* and
*FCFT\_SCALING\_FILTER\_BILINEAR* are traditional filters, with
_nearest_ being the fastest.

The rest are separable convolution filters.

The default is *FCFT\_SCALING\_FILTER\_CUBIC*.

# EMOJI PRESENTATION

_emoji\_presentation_ configures the default presentation style to use
with emojis with both a text presentation, and an emoji presentation
style.

These emojis can be followed by an (invisible) variation
selector. When they are not followed by a selector, the implementation
must choose a style.

One example of such an emoji is U+263A - WHITE SMILING FACE:

- unqualified: ☺
- text: ☺
- emoji: ☺️

The Unicode standard defines the default presentation styles, and this
is what fcft defaults to.

However, in some cases, the application may want to always use either
the text style, or the emoji style.

This is what _emoji\_presentation_ is for. Possible values are:

- *FCFT\_EMOJI\_PRESENTATION\_DEFAULT*
- *FCFT\_EMOJI\_PRESENTATION\_TEXT*
- *FCFT\_EMOJI\_PRESENTATION\_EMOJI*

To re-iterate; this setting affects emoji's without an explicit
variation selector. In other words, it overrides the *default*
presentation style.

*FCFT\_EMOJI\_PRESENTATION\_DEFAULT* is the default, and causes fcft
to use the default presentation as defined by Unicode.

*FCFT\_EMOJI\_PRESENTATION\_TEXT* forces all multi-presentation style
emojis to be rendered in their text presentation style.

*FCFT\_EMOJI\_PRESENTATION\_EMOJI* forces all multi-presentation style
emojis to be rendered in their emoji presentation style.

# COLOR GLYPHS

_srgb\_decode_: when true, the pixel data will be decoded from sRGB to
linear space. When false (the default), the pixel data is used as is.

_format_ defines the pixman surface format to use for color
glyphs. The default is *PIXMAN_a8r8g8b8*. One use is to enable higher
color precision when _srgb\_decode_ is enabled, to avoid loss of
precision when decoding from sRGB.

# SEE ALSO

*fcft_from_name2*(), *fcft_font_options_destroy*()
