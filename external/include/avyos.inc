script: |-
    cmake -S /avyos -B %{build-dir}     \
        -DCMAKE_INSTALL_PREFIX=%{prefix}          \
        -DCMAKE_INSTALL_SYSCONFDIR=%{sysconfdir}  \
        -DCMAKE_INSTALL_BINDIR=%{bindir}          \
        -DCMAKE_INSTALL_SBINDIR=%{sbindir}        \
        -DCMAKE_INSTALL_LIBDIR=%{libdir}          \
        -DCMAKE_INSTALL_LIBEXECDIR=%{libexecdir}  \
        -DCMAKE_INSTALL_LOCALSTATEDIR=%{localstatedir}  \
        -DCMAKE_INSTALL_DATADIR=%{datadir}        \
        -DCMAKE_BUILD_TYPE=Release                \
        -DCMAKE_MODULE_PATH=/avyos/cmake %{configure}

    cmake --build %{build-dir} %{compile} --target %{id}
    DESTDIR=%{install-root} cmake --install %{build-dir} %{install} --target %{id}