variables:
  include-root: "/sysroot/"
  apps-dir: "apps"
  include-depends: false
  force-rebuild: true
  strip: false

build-depends:
  - external/appimage
  - external/desktop-file-utils

script: |
  for appImage in /sysroot/*.app ; do
    appimage $appImage
  done
  mkdir -p %{install-root}%{datadir}/factory/
  mv /apps %{install-root}%{datadir}/factory/
  update-desktop-database %{install-root}%{datadir}/factory/apps/share/applications/

  install -vDm0644 /dev/stdin %{install-root}/%{libdir}/systemd/system/install-apps.service << "EOF"
  [Unit]
  Description=Install Builtin Apps
  ConditionPathNotExists=/apps

  [Service]
  Type=oneshot
  ExecStart=%{bindir}/cp -rap %{datadir}/factory/apps /

  [Install]
  WantedBy=multi-user.target
  EOF
