version: 14.2.0
release: 1
about: The GNU compiler collection, which includes the C and C++ compilers

pre-script: |-
  sed -e '/m64=/s/lib64/lib/' -i.orig gcc/config/i386/t-linux64

configure: >-
  --with-bugurl=https://github.com/itsmanjeet/avyos/issues
  --with-linker-hash-style=gnu
  --with-system-zlib
  --enable-__cax_atexit
  --enable-cet
  --enable-checking=release
  --enable-clocale=gnu
  --enable-default-pie
  --enable-default-ssp
  --enable-gnu-indirect-function
  --enable-gnu-unique-object
  --enable-libstdcxx-backtrace
  --enable-link-serialization=1
  --enable-linker-build-id
  --enable-lto
  --enable-plugin
  --enable-shared
  --enable-threads=posix
  --enable-languages=c,c++,lto
  --enable-bootstrap
  --disable-libssp
  --disable-libstdcxx-pch
  --disable-werror
  --disable-multilib

post-script: |
  ln -sv ../bin/cpp %{install-root}/%{libdir}/

  mkdir -p %{install-root}/%{libdir}/bfd-plugins/
  ln -sfv ../../lib/gcc/x86_64-pc-linux-gnu/%{version}/liblto_plugin.so \
    %{install-root}/%{libdir}/bfd-plugins/

  ln -sv gcc %{install-root}/usr/bin/cc

  if [[ -d %{install-root}/lib ]] ; then
    cp %{install-root}/lib/* %{install-root}/%{libdir}/ -a
    rm %{install-root}/lib -r
  fi

sources:
  - https://ftp.gnu.org/gnu/gcc/gcc-%{version}/gcc-%{version}.tar.xz
  - patches/gcc/fix-asan-allocator-aslr.patch
