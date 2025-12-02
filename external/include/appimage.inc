strip: false
skip-libraries: >-
  libc.so*

prepare-appimage: ""
type: app
build-depends:
- components/appimagetool.yml
- components/zsync.yml
- components/gnupg.yml
- components/appstream.yml

post-script: |
  install -vDm0755 %{libdir}/appimagekit/AppRun -t %{install-root}
  %{prepare-appimage}

  [ -d %{install-root}/%{datadir}/glib-2.0/schemas/ ] && glib-compile-schemas %{install-root}/%{datadir}/glib-2.0/schemas/
  
  for i in help man doc gtk-doc ; do
    [ -d %{install-root}/%{datadir}/${i} ] && rm -rf %{install-root}/%{datadir}/${i}
  done

  list_libraries() {
    echo "installing library $1"
    dependencies=$(ldd $1 | awk '{print $3}')

    for dep in $dependencies ; do
      grep "$dep" required-libraries && continue
      echo "$dep" >> required-libraries
      list_libraries "$dep"
    done
  }

  for i in $(find %{install-root}/ -type f -executable); do
    grep "$i" required-libraries && continue
    list_libraries $i
  done

  if [[ -n "%{skip-libraries}" ]] ; then
    skipped=""
    for i in %{skip-libraries}
    do
      skipped="$skipped -e $i"
    done
    FILTER="grep -v $skipped"
  else
    FILTER="cat"
  fi

  mkdir -p %{install-root}/%{libdir}
  for i in $(cat required-libraries | sort | uniq | ${FILTER}) ; do
    if [[ -e $i ]] ; then
      cp $i %{install-root}/%{libdir}
    elif [[ -e %{install-root}/$i ]] ; then
      continue
    else
      echo "MISSING REQUIRED LIBRARY $i"
    fi
  done

  if [ "$(find %{install-root}/ -maxdepth 1 -type f -name '*.desktop' 2>/dev/null | wc -l)" -eq "0" ] ; then
    cp -v %{install-root}/%{datadir}/applications/*.desktop %{install-root}/
  fi
  
  if [ "$(find %{install-root}/ -maxdepth 1 -type f -name '*.svg' 2>/dev/null | wc -l)" -eq "0" ] ; then
    for i in $(find %{install-root}/%{datadir}/icons/ -name "*.png" -o -name "*.svg") ; do
      cp -v ${i} %{install-root}/
    done
  fi

  mkdir -p AppDir/
  mv %{install-root}/* AppDir/

  cd %{install-root}/
  appimagetool -nu "zsync|http://repo.avyos.dev/apps/%{id}.app.zsync" %{build-root}/AppDir/ %{id}.app