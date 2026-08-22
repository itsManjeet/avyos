script: |-
  GOPATH="%{build-root}" go install %{gourl}@%{version}

  find %{build-root} \
    -name ".*" -prune -exec rm -r '{}' \;

  mkdir -p %{install-root}%{libdir}/go
  cp -Rv %{build-root}/{src,pkg} %{install-root}%{libdir}/go


build-depends:
  - external/go