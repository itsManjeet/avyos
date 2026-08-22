include-root: /sysroot
force-rebuild: false
include-upon: external/platform
strip: false
type: extension
extra-commands: ""
initial-commands: |-
  [ -d %{include-root}/%{sysconfdir} ] && mv %{include-root}/%{sysconfdir} %{include-root}/%{prefix}/
  %{extra-commands}

script: |-
  %{initial-commands}

  rm -f "%{include-root}/%{libdir}/os-release"
  install -d "%{include-root}/%{libdir}/extension-release.d"
  printf 'ID=rlxos\n' > "%{include-root}/%{libdir}/extension-release.d/extension-release.%{id}"

  rm -rf "%{build-root}/sysext-root"
  install -d "%{build-root}/sysext-root"
  for dir in usr opt; do
    [ -d "%{include-root}/${dir}" ] || continue
    cp -a "%{include-root}/${dir}" "%{build-root}/sysext-root/${dir}"
  done

  install -d "%{install-root}"
  mksquashfs "%{build-root}/sysext-root" "%{install-root}/%{id}.raw" -all-root -noappend

build-depends:
  - external/squashfs-tools

merge: [channel.yml]
