strip: false
force-rebuild: true
include-root: "/sysroot"
script: |-
  chroot %{include-root} /bin/bash -e << "EOT"
  for i in %{datadir}/pkgupd/manifest/*/integration ; do
    sh -e $i
  done

  %{initial-commands}
  EOT

  mv %{include-root} %{install-root}