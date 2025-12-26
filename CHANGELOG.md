# Changelog

* [Unreleased](#unreleased)
* [1.3.0](#1-3-0)
* [1.2.0](#1-2-0)
* [1.1.0](#1-1-0)


## Unreleased
### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security
### Contributors


## 1.3.0

### Added

* Nanosvg updated to ea6a6aca009422bba0dbad4c80df6e6ba0c82183
* JPEG XL support ([#14][14])
* Log output now respects the [`NO_COLOR`](http://no-color.org/)
  environment variable.
* Support for linking against a system provided nanosvg library. See
  the new `-Dsystem-nanosvg` meson option. Defaults to `disabled`
  (i.e. use the bundled version).
* SVG images rendered at output resolution; this prevents them from
  being unnecessarily blurry.
* Flags added to stretch wallpapers: `[-s|--stretch]` ([#13][13])


[14]: https://codeberg.org/dnkl/wbg/pulls/14
[13]: https://codeberg.org/dnkl/wbg/issues/13


### Changed

* "Centered maximized" is the default method now ([#13][13])
* Decreased run-time memory usage, by unloading image data after
  rendering.


### Fixed

* Alpha not being applied correctly to PNG images.
* Compilation error on musl libc ([#11][11]).

[11]: https://codeberg.org/dnkl/wbg/issues/11


### Contributors

* Leonardo Hernández Hernández
* emrakyz
* Sertonix


## 1.2.0

### Added

* We now hint to the compositor that the background is fully opaque.
* SVG support.


### Changed

* Image is now zoomed, rather than stretched ([#6][6]).

[6]: https://codeberg.org/dnkl/wbg/issues/6


### Fixed

* Respect the `layer_surface::closed()` event.


### Contributors

* Leonardo Hernández Hernández
* sewn


## 1.1.0

### Added

* webp support


### Fixed

* meson: can’t use `SOURCE_DIR` in `custom_targets()`.
* build: version script is now run in a C locale.
* Don’t re-render frame on same-size configure events


### Contributors

*  Leonardo Hernández
