include-root: "/sysroot"
strip: false
force-rebuild: true

script: |-
  %{initial-commands}
  ostree init --repo %{build-root}/barerepo
  ostree commit --repo %{build-root}/barerepo --consume %{include-root} --branch %{ostree-branch} --timestamp 2011-11-11 11:11:11+00:00

  mkdir -p %{install-root}
  ostree init --repo %{install-root} --mode archive
  ostree pull-local --repo %{install-root} %{build-root}/barerepo


build-depends:
  - external/ostree
  - external/grub
  - external/dracut