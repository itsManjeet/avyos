# Changelog

* [Unreleased](#unreleased)
* [3.3.2](#3-3-2)
* [3.3.1](#3-3-1)
* [3.3.0](#3-3-0)
* [3.2.0](#3-2-0)
* [3.1.10](#3-1-10)
* [3.1.9](#3-1-9)
* [3.1.8](#3-1-8)
* [3.1.7](#3-1-7)
* [3.1.6](#3-1-6)
* [3.1.5](#3-1-5)
* [3.1.4](#3-1-4)
* [3.1.3](#3-1-3)
* [3.1.2](#3-1-2)
* [3.1.1](#3-1-1)
* [3.1.0](#3-1-0)
* [3.0.1](#3-0-1)
* [3.0.0](#3-0-0)
* [2.5.1](#2-5-1)
* [2.5.0](#2-5-0)
* [2.4.6](#2-4-6)
* [2.4.5](#2-4-5)
* [2.4.4](#2-4-4)
* [2.4.3](#2-4-3)
* [2.4.2](#2-4-2)
* [2.4.1](#2-4-1)
* [2.4.0](#2-4-0)
* [2.3.3](#2-3-3)
* [2.3.2](#2-3-2)
* [2.3.1](#2-3-1)
* [2.3.0](#2-3-0)
* [2.2.7](#2-2-7)
* [2.2.6](#2-2-6)
* [2.2.5](#2-2-5)
* [2.2.4](#2-2-4)
* [2.2.3](#2-2-3)
* [2.2.2](#2-2-2)
* [2.2.1](#2-2-1)
* [2.2.0](#2-2-0)
* [2.1.3](#2-1-3)
* [2.1.2](#2-1-2)
* [2.1.1](#2-1-1)
* [2.1.0](#2-1-0)
* [2.0.0](#2-0-0)
* [1.1.7](#1-1-7)


## Unreleased
### Added
### Changed

* Failure to set LCD filter is no longer fatal


### Deprecated
### Removed
### Fixed
### Security
### Contributors


## 3.3.2

### Fixed

* Crash when failing to load a font.
* Memory leak in `fcft_fini()` if the font cache contained entries for
  fonts that failed to load.


### Contributors

* markstos


## 3.3.1

### Fixed

* Regression: binary incompatibility with fcft <= 3.2, in
  `fcft_set_scaling_filter()` ([#1992][foot-1992]).

[foot-1992]: https://codeberg.org/dnkl/foot/issues/1992


## 3.3.0

### Added

* `scaling_filter` to `fcft_font_options`. The default value is
  `FCFT_SCALING_FILTER_CUBIC`.
* New scaling filters: impulse, box, linear, gaussian, lanczos3 and
  lanczos3_stretched.


### Changed

* When scaling bitmap fonts (not emoji fonts), do not apply a scaling
  filter. In earlier versions, fcft applied a bilinear scaling filter.


### Deprecated

* `fcft_set_scaling_filter()`; use `fcft_font_options::scaling_filter`
  instead.


## 3.2.0

### Added

* Nanosvg updated to ea6a6aca009422bba0dbad4c80df6e6ba0c82183
* `fcft_from_name2()`, allowing you to instantiate fonts and override
  certain rendering defaults.
* `fcft_font_options_create()` and `fcft_font_options_destroy()`, to
  be used with the new `fcft_from_name2()`.
* fcft can now decode color glyphs from sRGB to linear, and optionally
  render to higher precision pixman surfaces. See the new
  `fcft_from_name2()` and `fcft_font_options_create()`.


### Changed

* Unicode codepoints with the _emoji components_ tag are no longer
  treated as emojis, when selecting font. This fixes an issue where
  e.g. `0..9` were loaded from a fallback font instead of the primary
  font, if the primary font had the _emoji_ language bit set
  ([#1978][foot-1978]).

[foot-1978]: https://codeberg.org/dnkl/foot/issues/1978


### Deprecated

* `fcft_set_emoji_presentation()` - use `fcft_from_name2()` instead.


### Fixed

* Undefined behavior when using copies of `mtx_t` and `pthread_rwlock_t`
  objects.


### Contributors

* CismonX


## 3.1.10

### Added

* Unicode data files updated to Unicode 16. Fcft uses these files to
  determine the default presentation (textual or graphical) for
  emojis, and to generate pre-compose tables.


### Changed

* Use `utf8proc_charwidth()` instead of `wcwdith()` to calculate
  character (terminal) width, when building with utf8proc support.


## 3.1.9

### Added

* Nanosvg updated to 93ce879dc4c04a3ef1758428ec80083c38610b1f
* Unicode data files updated to 15.1
* `FCFT_LOG_COLORIZE_AUTO` now respects the `NO_COLOR` environment
  variable.
* Support for linking against a system provided nanosvg library. See
  the new `-Dsystem-nanosvg` meson option. Defaults to `disabled`
  (i.e. use the bundled version).


### Changed

* Positioning of SVG glyphs; fcft now tries to center them on the
  line.


### Fixed

* Log-level not respected by syslog
* Fcft now automatically ensures the `LC_NUMERIC` locale is correctly
  configured for `FcNameParse()` ([#60][60]).
* Failure to lookup font name (_"warn: fcft.c:583: failed to get full
  font name"_)

[60]: https://codeberg.org/dnkl/fcft/issues/60


### Contributors

* Jake Waksbaum


## 3.1.8

### Changed

* Set font size using `FT_Set_Char_Size()` instead of
  `FT_Set_Pixel_Sizes()`, as it provides a more fine-grained
  interface, resulting in a more exact font size ([#1621][foot-1621]).

[foot-1621]: https://codeberg.org/dnkl/foot/issues/1621


## 3.1.7

### Fixed

* Infinite loop on certain invalid `fontfeatures` values
  ([#1554][foot-1554], [#1556][foot-1556]).

[foot-1554]: https://codeberg.org/dnkl/foot/issues/1554
[foot-1556]: https://codeberg.org/dnkl/foot/issues/1556


## 3.1.6

### Added

* nanosvg updated to 9da543e (2022-12-04)


## 3.1.5

### Changed

* SVG: user transformations are now ignored, instead of returning
  _”unimplemented feature”_ error.


### Fixed

* Reverted “fixed: bitmap font glyph advance width calculation”.

  It appears that HarfBuzz <= 5.1 calculated glyph advance widths
  incorrectly for bitmap fonts, when bitmap font scaling has been
  enabled (`pixelsizefixupfactor` has been set in FontConfig).

  fcft-3.1.3 compensated for this when calculating the glyphs’ advance
  widths.

  HarfBuzz >= 5.2 changed how it calculates the advance widths, and
  fcft’s compensation now causes and excessive advance width.

  Thus, the patch from fcft-3.1.3 has been reverted.

  [#163][fuzzel-163]

[fuzzel-163]: https://codeberg.org/dnkl/fuzzel/issues/163


## 3.1.4

### Fixed

* Crash when failing to load an SVG glyph with multiple sub-glyphs.


## 3.1.3

### Fixed

* Bitmap font glyph advance width calculation in
  `fcft_rasterize_grapheme_utf32()` and
  `fcft_rasterize_text_run_utf32()` when user had enabled
  `10-scale-bitmap-fonts.conf` in FontConfig.


## 3.1.2

### Fixed

* SVG glyphs failing with “bad argument” if LCD subpixel rendering is
  enabled ([#1069][foot-1069]).

[foot-1069]: https://codeberg.org/dnkl/foot/issues/1069


## 3.1.1

### Fixed

* Crash when multiple SVG glyphs are rasterized in parallel (i.e. the
  SVG backend is now thread safe) ([#51][51]).
* Crash when rasterizing SVG glyphs with transforms (e.g. synthetic
  slanting). Transforms are now completely ignored ([#51][51]).

[51]: https://codeberg.org/dnkl/fcft/issues/51


## 3.1.0

### Added

* OT-SVG support ([#49][49]). Note that FreeType does not rasterize
  SVG glyphs by itself. Instead, fcft does this via FreeType hooks,
  using a bundled [nanosvg][nanosvg] as backend. Enabled by default,
  but can be disabled with `-Dsvg-backend=none`. FreeType >= 2.12 is
  required.
* `FCFT_CAPABILITY_SVG` added to `fcft_capabilities()`.

[nanosvg]: https://github.com/memononen/nanosvg
[49]: https://codeberg.org/dnkl/fcft/issues/49


### Changed

* Minimum required meson version is now 0.58


### Fixed

* `fcft_font::name`, and `fcft_glyph::font_name` not being set
  correctly for font collections (e.g. `*.ttc` files).
* Assertion inside HarfBuzz with `fcft_rasterize_grapheme_ut32()` when
  we fail to rasterize a glyph ([#1056][foot-1056]).

[foot-1056]: https://codeberg.org/dnkl/foot/issues/1056


## 3.0.1

### Fixed

* Crash when failing to lookup the full font name
  (https://codeberg.org/dnkl/fcft/issues/47).


## 3.0.0

### Added

* `name` to `struct fcft_font`. This is the name of the **primary**
  font (i.e. not any of the fallback fonts).
* `font_name` to `struct fcft_glyph`. This is the name of the font the
  glyph was loaded from, and may reference a fallback font. Note that
  it is **always** NULL in text-run glyphs.


### Changed

* meson: default C standard changed from C18 to C11 (fcft does not use
  any C18 features).
* fcft’s constructor and destructor are now part of the public API,
  and must be explicitly called: `fcft_init()` and `fcft_fini()`. Note
  that `fcft_init()` also handles logging initialization.
* All `wchar_t` usage replaced with `uint32_t`.
* layout tags removed from `fcft_grapheme_rasterize()`. These should
  be set using `fontfeature`s in the font `names` and/or `attributes`
  arguments in `fcft_from_name()`.
* `wc` member in `struct glyph` renamed to `cp` (CodePoint).
* `wc` function arguments renamed to `cp` in all APIs.
* `fcft_glyph_rasterize()` renamed to `fcft_rasterize_char_utf32()`.
* `fcft_grapheme_rasterize()` renamed to `fcft_rasterize_grapheme_utf32()`.
* `fcft_text_run_rasterize()` renamed to `fcft_rasterize_text_run_utf32()`.


### Removed

* `fcft_size_adjust()`
* `space_advance` member from `struct fcft_font`.
* `fcft_log_init()`. Use `fcft_init()` instead.


### Fixed

* Wrong version string generated when building as a subproject.
* Uninitialized grapheme state variable in
  `fcft_text_run_rasterize()`.
* Incorrectly sorted Unicode precompose table when building in non-C
  locales (https://codeberg.org/dnkl/fcft/issues/44).


## 2.5.1

### Added

* Meson command line option `-Ddocs` to force disable or enable
  building manual pages and installing changelog and readme files


### Changed

* `scdoc` is optional and detected automatically
* `fcft_set_scaling_filter()` now applies to color bitmap fonts only
  (i.e. emoji fonts). Applying e.g. cubic or lanczos3 on regular text
  glyphs simply does not look good.


### Fixed

* `FCFT_SCALING_FILTER_CUBIC` incorrectly being mapped to “lanczos3”.
* Pixman errors and program freezes when scaling bitmap fonts to very
  small sizes (https://codeberg.org/dnkl/foot/issues/830).


### Contributors

*  Alibek Omarov


## 2.5.0

### Added

* Text-run shaping now requires libutf8proc (in addition to
  HarfBuzz). Grapheme shaping still requires HarfBuzz only.
* `fcft_set_emoji_presentation()` - can be used to override emojis’
  **default** presentation style.


### Changed

* Meson command line option `-Dtext-shaping` have been replaced with
  `-Dgrapheme-shaping` and `-Drun-shaping`; grapheme shaping requires
  HarfBuzz only, while run shaping requires HarfBuzz **and**
  libutf8proc. Thus, enabling run shaping implicitly enables grapheme
  shaping.
* `fcft_*_rasterize()`: emojis’ default presentation is now accounted
  for when searching for a font containing the emoji codepoint;
  codepoints whose default presentation is “text” will no longer
  consider emoji fonts, and codepoints whose default presentation is
  “emoji” will no longer consider non-emoji fonts.


### Fixed

* `fcft_text_run_rasterize()`: much improved handling of RTL scripts
  (in mixed LTR/RTL strings in particular)


## 2.4.6

### Added

* UnicodeData updated to 14.0


### Fixed

* ‘cp’ field in `struct fcft_glyph` being assigned font index instead
  of Unicode codepoint in `fcft_grapheme_rasterize()` and
  `fcft_text_run_rasterize()`.
* Assertion in `glyph_cache_resize()`, triggered by trying to
  rasterize, and _failing_, a large amount of code points
  (https://codeberg.org/dnkl/foot/issues/763).
* Bad performance of grapheme cache when rasterizing many grapheme
  clusters.


### Contributors

* [emersion](https://codeberg.org/emersion)


## 2.4.5

### Fixed

* `fcft_text_run_rasterize()` not checking if codepoint is in the
  font’s charset before attempting shaping
  (https://codeberg.org/dnkl/fcft/issues/30).
* Crash when destroying a font with a grapheme cache entry
  representing a failed grapheme glyph.


## 2.4.4

### Fixed

* Rendering of bitmap fonts with Freetype >= 2.11
  (https://codeberg.org/dnkl/fcft/issues/29).


## 2.4.3

### Fixed

* Bitmap/aliased font glyphs being mirrored on big-endian
  architectures.
* Color font glyphs having wrong colors on big-endian architectures.
* Crash when destroying a font that failed to load (typically happens
  when there are no fonts available at all).


## 2.4.2

### Fixed

* Rare crash when one thread was doing a glyph cache lookup, while
  another was resizing the cache.


## 2.4.1

### Changed

* Log messages are now printed to stderr instead of stdout.
* `fcft_grapheme_rasterize()` now sets a minimum grapheme column count
  of 2 when the cluster ends with an Emoji variant selector (codepoint
  0xFE0F).


### Fixed

* Compilation error when fallback definition for `FCFT_EXPORT` was used
  in `meson.build`.


### Contributors

* [emersion](https://codeberg.org/emersion)
* [craigbarnes](https://codeberg.org/craigbarnes)


## 2.4.0

### Added

* Example program. Very simple bare bones Wayland program that renders
  a user provided string with user configurable fonts and colors. No
  proper error checking etc. To build, configure meson with
  `-Dexamples=true`.
* `fcft_log_init()`. This function enables, and configures logging in fcft.
* `fcft_text_run_rasterize()`: new API that uses HarfBuzz to shape a
  text run (i.e. a whole string). Note that HarfBuzz is (still) an
  **optional** dependency, see
  [README](README.md#user-content-building).
* `fcft_text_run_destroy()`: new API that frees a rasterized text-run.
* `FCFT_CAPABILITY_TEXT_RUN_SHAPING` added to `fcft_capabilities()`.
* `antialias` and `subpixel` members to `struct fcft_font`.


### Changed

* fcft logging must now be enabled explicitly (see `fcft_log_init()`).
* Internal logging functions are no longer exported by the shared library.
* The pixel size passed from FontConfig to FreeType is now rounded
  instead of truncated (https://codeberg.org/dnkl/foot/issues/456).


### Fixed

* Internal logging functions have been renamed, from generic `log_*`
  names to fcft specific `fcft_log_` names.
* Apply pixel-size fixup to glyphs’ advance width/height, but **only**
  if we estimated the fixup ourselves (otherwise the advance
  width/height is already scaled).


## 2.3.3

### Fixed

* Cloned fonts not being properly freed in library destructor.


## 2.3.2

### Added

* Limited support for _"fontfeatures_" . fcft is still not a layout
  engine, but with this we can support e.g. _stylistic sets_
  (HarfBuzz-enabled builds only).


### Deprecated

* `tags` argument in `fcft_grapheme_rasterize()`. It is now being
  ignored, and will be removed in a future release.


### Fixed

* Hang in library destructor when system has no fonts installed
  (https://codeberg.org/dnkl/foot/issues/174).


### Contributors

* [birger](https://codeberg.org/birger)


## 2.3.1

### Fixed

* `fcft_grapheme_rasterize()` now makes use of the optional
  `tags`. These were previously ignored.
* Compilation error when text shaping was disabled.


## 2.3.0

### Added

* `fcft_set_scaling_filter()`: new API that lets the calling
  application configure the filter to use when downscaling bitmap
  fonts (https://codeberg.org/dnkl/fcft/issues/9).
* `fcft_grapheme_rasterize()`: new API that uses HarfBuzz to shape a
  grapheme cluster. Note that HarfBuzz is an **optional** dependency,
  see [README](README.md#user-content-building).


### Changed

* Increased timeout in tests, from 4s (the default), to 60s
  (https://codeberg.org/dnkl/foot/issues/120).


## 2.2.7

### Changed

* Use lanczos3 filtering when downscaling bitmap fonts. This improves
  the quality of e.g. color emoji fonts.


### Fixed

* Compilation with `-pedantic`


## 2.2.6

### Fixed

* Set LCD filter. This fixes **severe** color fringes when FreeType
  has been built with `FT_CONFIG_OPTION_SUBPIXEL_RENDERING` (i.e. the
  old ClearType-style subpixel rendering, instead of the newer Harmony
  LCD rendering).


## 2.2.5

### Changed

* `fcft_glyph_rasterize()`: improved performance in threaded
  applications by guarding the glyph cache with an _rwlock_ instead of
  a _mutex_.


### Fixed

* fcft now checks for memory allocation failures.
* Compilation errors in 32-bit builds.


## 2.2.4

### Added

* Unicode license file, for `UnicodeData.txt`


## 2.2.3
### Added

* Missing [LICENSE](LICENSE) file
* [LICENSE](LICENSE), [README.md](README.md) and
  [CHANGELOG.md](CHANGELOG.md) are now installed to
  `${datadir}/doc/fcft`.


### Fixed

* Assertion in debug builds when resizing the glyph cache


## 2.2.2

### Fixed

* `fcft_kerning()` was not threadsafe
* Rare crash in `fcft_glyph_rasterize()` caused by a race between a
  successful glyph cache lookup and a glyph cache resize.


## 2.2.1

### Changed

* Color bitmap glyphs with a pixel-size fixup factor other than 1.0
  are now pre-scaled. Previously, fcft would only set a pixman scale
  transform on the glyph, causing actual scaling to occur **every**
  time the glyph was blended. This improves the performance when
  rendering color emojis.


## 2.2.0

### Changed

* Internal representation of the primary and fallback fonts.
* Do not load a fallback font if it does not contain the requested glyph.


### Deprecated

* `fcft_size_adjust()`


## 2.1.3

### Fixed

* Advance width and height of scaled bitmap fonts.


## 2.1.2

### Changed

* Glyph cache now resizes dynamically. This fixes a performance
  problem when loading **a lot** of glyphs, as we ended up scanning
  very long lists when looking up a glyph in the cache.


## 2.1.1

### Changed

* Prefer user-provided `charset`. This can be used to e.g. limit a
  fallback font's usage to a custom Unicode point range.


### Fixed

* LCD RGB/BGR modes were reversed.


## 2.1.0

### Added

* When looking for a glyph in the fallback fonts, don't discard
  (destroy/unload) the fonts that did not contain the glyph. This
  improves performance massively when loading lots of glyphs that does
  not exist in the primary font, or in the first fallback font(s).
* Synthetic bold and italics (FontConfig's _embolden_ and _matrix_
  properties).
* `fcft_precompose()` - combines a base- and a combining wide
  character into a single pre-composed character.


### Changed

* `fcft_from_name()` and `fcft_size_adjust()` no longer calls
  `setlocale()` to set a suitable locale for `FcParseName()`, as this
  was not thread safe. The caller is responsible for ensuring
  `LC_NUMERICAL` is set to a locale that correctly recognizes _x.y_
  decimal values.


### Fixed

* `fcft_from_name()` was not thread safe
* `fcft_clone()` was not thread safe
* `fcft_size_adjust()` was not thread safe
* `fcft_destroy` was not thread safe


## 2.0.0

### Changed

* API: `font_` prefix changed to `fcft_`.
* API: renamed `struct font` to `struct fcft_font`.
* API: renamed `struct glyph` to `struct fcft_glyph`.
* API: internal members of `struct fcft_glyph` removed.
* API: renamed `enum subpixel_order` to `enum fcft_subpixel`, and
  `ORDER` was removed from the enum values.
* API: renamed `fcft_glyph.x_advance` to `fcft_glyph.advance.x`, and
  added `fcft_glyph.advance.y`
* API: renamed `fcft_font.max_x_advance` to `fcft_font.max_advance.x`
  and added `fcft_font.max_advance.y`.
* API: renamed `fcft_font.space_x_advance` to
  `fcft_font.space_advance.x` and added `fcft_font.space_advance.y`.
* API: renamed `fcft_glyph_for_wc()` to `fcft_glyph_rasterize()`.
* Require meson >= 0.54.
* Use `meson.override_dependency()`.


### Fixed

* `fcft_kerning()` did not scale the returned kerning distances with
  the font's pixel size fixup multiplier.


## 1.1.7

### Fixed

* LCD RGB/BGR modes were reversed
