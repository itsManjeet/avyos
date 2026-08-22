extra-commands: ""

script: |-
    ( cd /rlxos; go build -o %{install-root}/%{bindir}/%{id} rlxos/apps/%{id} )
    %{extra-commands}

post-script: |-
    go clean -modcache
    rm -rf /go

build-depends:
    - external/go

depends:
    - external/gtk4
