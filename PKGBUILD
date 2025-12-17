pkgname=fcft
pkgver=3.3.2
pkgrel=1
pkgdesc="Simple font loading and glyph rasterization library"
changelog=CHANGELOG.md
arch=('x86_64' 'aarch64')
url=https://codeberg.org/dnkl/fcft
license=(mit)
makedepends=('meson' 'ninja' 'tllist>=1.0.1' 'scdoc')
depends=('freetype2' 'fontconfig' 'pixman' 'nanosvg')
checkdepends=('check' 'ttf-dejavu')
source=()

pkgver() {
  cd ../.git &> /dev/null && git describe --tags --long | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g' ||
      head -3 ../meson.build | grep version | cut -d "'" -f 2
}

build() {
  meson --prefix=/usr --buildtype=release --wrap-mode=nofallback -Dsystem-nanosvg=enabled ..
  ninja
}

check() {
  ninja test
}

package() {
  DESTDIR="${pkgdir}/" ninja install
}
