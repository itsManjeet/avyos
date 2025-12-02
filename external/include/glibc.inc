version: 2.41
about: GNU C Library

keep-files:
  - lib(.*)\.dbg

pre-script: |-
  echo "slibdir=%{libdir}"      >> configparms
  echo "rtlddir=%{libdir}"      >> configparms
  echo "sbindir=%{bindir}"      >> configparms
  echo "rootsbindir=%{bindir}"  >> configparms

configure: >-
  --with-headers=%{prefix}/include
  --with-bugurl=https://github.com/itsmanjeet/avyos/issues
  --enable-bind-now
  --enable-cet
  --enable-fortify-source
  --enable-kernel=4.4
  --enable-multi-arch
  --enable-stack-protector=strong
  --disable-nscd
  --disable-profile
  --disable-werror

compile: -O

post-script: |
  make -C %{build-root}/localedata objdir=%{build-root}/%{build-dir} \
    DESTDIR=%{build-root}/locales install-locale-files
  
  rm -f %{install-root}/%{sysconfdir}/ld.so.cache
  rm -f %{install-root}/%{bindir}/{tzselect,zdump,zic}
  
  install -dm755 %{install-root}/usr/lib/{locale,systemd/system,tmpfiles.d}
  
  install -m644 posix/gai.conf %{install-root}/%{sysconfdir}/gai.conf
  
  sed -e '1,3d' -e 's|/| |g' -e 's| \\||g' \
    localedata/SUPPORTED > %{install-root}/%{datadir}/i18n/SUPPORTED

  install -dm755 %{install-root}/%{libdir}/locale
  cp -r %{build-root}/locales/%{libdir}/locale/C.utf8 -t %{install-root}/%{libdir}/locale

  install -Dm644 sdt.h %{install-root}/%{prefix}/include/sys/sdt.h
  install -Dm644 sdt-config.h %{install-root}/%{prefix}/include/sys/sdt-config.h

sources:
  - https://ftp.gnu.org/gnu/glibc/glibc-%{version}.tar.xz
  - files/sdt.h
  - files/sdt-config.h
  
