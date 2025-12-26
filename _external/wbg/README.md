# Wbg

Super simple wallpaper application for Wayland compositors
implementing the layer-shell protocol.

Wbg takes a single command line argument: a path to an image
file. This image is displayed scaled-to-fit on all monitors.

More display options, and/or the ability to set a per-monitor
wallpaper _may_ be added in the future.

[![Packaging status](https://repology.org/badge/vertical-allrepos/wbg.svg?columns=4)](https://repology.org/project/wbg/versions)


## Requirements

### Runtime

* pixman
* wayland (_client_ and _cursor_ libraries)
* libpng (optional)
* libjpeg (optional)
* libwebp (optional)
* libjxl (optional)
* libjxl_threads (optional)

Note that if SVG support is disabled at least one of _libpng_, _libjpeg_,
_libwebp_ and _libjxl_ is required.

_libjxl\_threads_ is recommended for better performance decoding JPEG XL images


### Compile time

* Development packages for all the libraries listed under _runtime_.
* wayland-protocols
* [tllist](https://codeberg.org/dnkl/tllist)


## Building

```sh
meson --buildtype=release build
ninja -C build
sudo ninja -C build install
```

By default, PNG, JPEG, JPEG XL and WebP support is auto-detected. You can force
disable/enable them with the meson command line options
`-Dpng=disabled|enabled`, `-Djpeg=disabled|enabled` and
`-Dwebp=disabled|enabled` `-Djxl=disabled|enabled`.
SVG support is enabled by default (as it does not require additional
dependencies). You can disable it with the meson command line option
`-Dsvg=false`


## Derivative work

* https://codeberg.org/droc12345/wbg - adds support for directories
  with images, random, timer flags.


## License
Wbg is released under the [MIT license](LICENSE).

Wbg (optionally) uses nanosvg, released under the [Zlib
license](3rd-party/nanosvg/LICENSE.txt).
