variables:
  strip: false

post-script: |-
  for i in %{install-root}%{datadir}/icons/* ;do
    gtk-update-icon-cache ${i}
  done