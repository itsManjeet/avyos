pkgname=tllist
pkgver=1.1.0
pkgrel=1
pkgdesc="A C header file only implementation of a typed linked list"
arch=('x86_64' 'aarch64')
url=https://codeberg.org/dnkl/tllist
license=(mit)
makedepends=('meson' 'ninja')
depends=()
source=()

pkgver() {
  cd ../.git &> /dev/null && git describe --tags --long | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g' ||
      head -3 ../meson.build | grep version | cut -d "'" -f 2
}

build() {
  meson --prefix=/usr --buildtype=release ..
  ninja
}

package() {
  DESTDIR="${pkgdir}/" ninja install
}
