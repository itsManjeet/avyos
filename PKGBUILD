pkgname=wbg
pkgver=1.3.0
pkgrel=1
pkgdesc="Super simple wallpaper application"
arch=('x86_64' 'aarch64')
url=https://codeberg.org/dnkl/wbg
license=(mit)
makedepends=('meson' 'ninja' 'wayland-protocols' 'tllist>=1.0.1')
depends=('wayland' 'pixman' 'nanosvg' 'libjpeg-turbo' 'libpng' 'libwebp' 'libjxl')
source=()
changelog=CHANGELOG.md

pkgver() {
  cd ../.git &> /dev/null && git describe --tags --long | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g' ||
      head -3 ../meson.build | grep version | cut -d "'" -f 2
}

build() {
  meson --prefix=/usr --buildtype=release --wrap-mode=nofallback -Dsystem-nanosvg=enabled ..
  ninja
}

package() {
  DESTDIR="${pkgdir}/" ninja install
}
