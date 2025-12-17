[![CI status](https://ci.codeberg.org/api/badges/dnkl/fcft/status.svg)](https://ci.codeberg.org/dnkl/fcft)

# fcft

[![Packaging status](https://repology.org/badge/vertical-allrepos/fcft.svg?columns=4)](https://repology.org/project/fcft/versions)

**fcft** is a small font loading and glyph rasterization library built
on-top of FontConfig, FreeType2 and pixman.

It is aimed at applications that value speed and simplicity. Its only
dependencies are those listed above, and optionally nanosvg (for
OT-SVG support). Fcft is perfect for pixman based applications, since
its "output" (the rasterized glyphs) are pixman images.

The primary goal of fcft is speed. It was created to support
[foot](https://codeberg/dnkl/foot), for which I found Cairo to be too
slow.

To achieve this, fcft has first class support for threaded glyph
lookup, and it aggressively caches rasterized glyphs. This is what
allows foot to make full use of its multiple worker (rendering)
threads.

Feature wise, it sits somewhere between Cairo and pango. Compared to
Cairo, it does **not** do generic 2D graphics, but it **does** do more
advanced font shaping.

Compared to pango, its font shaping support is more limited, but
instead it has better support for different font file formats (pango
only supports the font formats supported by HarfBuzz, while fcft
supports those handled by FreeType - and FreeType supports more font
formats than HarfBuzz).

It can load and cache fonts from a fontconfig-formatted name string,
e.g. `Monospace:size=12`, optionally with user configured fallback fonts.

After a font has been loaded, you can rasterize glyphs. When doing so,
the primary font is first considered. If it does not have the
requested glyph, the user configured fallback fonts (if any) are
considered. If none of the user configured fallback fonts has the
requested glyph, the FontConfig generated list of fallback fonts are
checked.

The API is documented as man pages. These are built and installed when
fcft is built as a standalone project, but **not** when built as a
subproject.

Starting with 2.3.0, fcft gained the ability to do text shaping using
HarfBuzz (an **optional** dependency). Initially, only individual
grapheme clusters could be shaped. With 2.4.0, the ability to shape
whole text-runs were added. It should be noted however, that fcft does
_not_ aim to compete with e.g. Pango, and that the text shaping
support may not be (and perhaps never will be) sufficient for
“professional” text layout.

![screenshot](screenshot.png "Screenshot showing the three rasterization methods")

The screenshot above is the output of the example program shipped with
the fcft sources (but not built by default). It showcases the three
different levels, at which fcft can rasterize glyphs:

Top: `fcft_glyph_rasterize()`
: Glyphs are rasterized character by character; no ligatures, no
  grapheme shaping, and right-to-left scripts are not rendered
  correctly.

Middle: `fcft_grapheme_rasterize()`
: Glyphs are rasterized grapheme by grapheme. Here, graphemes are
  shaped correctly (e.g. the family emoji), but ligatures and RTL
  scripts are still not rendered correctly.

Bottom: `fcft_text_run_rasterize()`
: The entire string is rasterized in one go. This means full shaping
  is applied; ligatures, graphemes, and RTL scripts.


## Requirements

* fontconfig
* freetype (>= 2.12 required for SVG support)
* pixman
* harfbuzz (optional, for grapheme and “run” shaping support)
* libutf8proc (optional, for “run” shaping support)
* [tllist](https://codeberg.org/dnkl/tllist), _unless_ built as a subproject


## Features

* Supports all fonts loadable by FreeType2
* Antialiasing
* Subpixel antialiasing
* Color bitmap fonts (_emoji_ fonts)
* OT-SVG (requires FreeType >= 2.12)
* Font caching
* Glyph caching
* Kerning[^1]
* Basic text shaping if compiled with HarfBuzz support

[^1]: only basic kerning supported (i.e. the old 'kern' tables, not
    the new 'GPOS' tables)


## Not supported

* Subpixel positioning

Remember, this is a _simple_ library, not a full blown layout engine.


## Projects using fcft

* [foot](https://codeberg.org/dnkl/foot)
* [yambar](https://codeberg.org/dnkl/yambar)
* [fuzzel](https://codeberg.org/dnkl/fuzzel)
* [fnott](https://codeberg.org/dnkl/fnott)


## Integrating

You can either install fcft as a system library, or use it as a meson
subproject (assuming your project is meson based, of course).


### Installing

If you install fcft as a system library, you can use `pkg-config` to
get the compiler flags needed to find and link against fcft.


### Meson

If your project is meson based, you can use fcft as a subproject. In
your main project's `meson.build`, do something like:

```meson
fcft = subproject('fcft').get_variable('fcft')
executable('you-executable', ..., dependencies: [fcft])
```

Or, if fcft has been installed as a system library, a regular

```meson
fcft = dependency('fcft')
```

will suffice. Optionally, you can combine the two; search for a system
library first, and fallback to a subproject:

```meson
fcft = dependency('fcft', version: '>=0.4.0', fallback: 'fcft')
```

## Building

Run-time dependencies:

* fontconfig
* freetype2
* pixman
* harfbuzz (_optional_, for text shaping support)
* utf8proc[^2]
* wayland[^2]
* wayland-protocols[^2]

[^2]: required by example programs

Build dependencies:

* Development packages of the run-time dependencies
* meson
* ninja
* scdoc (_optional_, enabled automatically or disabled with `-Ddocs=disabled`)
* [tllist](https://codeberg.org/dnkl/tllist)
* [check](https://libcheck.github.io/check/) (_optional_, for unit tests)

For most users, this is typically enough:
```sh
meson build --buildtype=release
ninja -C build
ninja -C build test
sudo ninja -C build install
```

The tests require at least **one** latin font to be installed.

By default, fcft will be built with support for **text-shaping** if
_HarfBuzz_ is available. You can explicitly enable or disable this
with the `-Dtext-shaping=disabled|enabled|auto` meson command line
option.

If text-shaping is enabled, you might also want to enable the
associated tests. Use `-Dtest-text-shaping=true` to do so. Note that
these tests require an emoji font to be installed, and `fc-match
emoji` must return that font first.

SVG support is also enabled by default, using the bundled
[nanosvg](https://github.com/memononen/nanosvg) library. You can
disable this with `-Dsvg-backend=none`.

To build the example programs, use the `-Dexamples=true` meson command
line option.


## License

fcft is released under the [MIT license](LICENSE).

fcft uses Unicode data files released under the [Unicode License
V3](https://www.unicode.org/license.txt).


# Sponsoring/donations

* Liberapay: https://liberapay.com/dnkl
* GitHub Sponsors: https://github.com/sponsors/dnkl

[![Donate using Liberapay](https://liberapay.com/assets/widgets/donate.svg)](https://liberapay.com/dnkl/donate)
