script: |-
  chroot %{sysroot} /bin/bash << "EOT"
  if [ -d %{localstatedir}/lib/integrations ] ; then
    for i in %{localstatedir}/lib/integrations/* ; do
      echo "=> integrating $(basename $i)"
      sh -e $i
    done
  fi

  mkinitramfs -u -k=$(ls -1 %{libdir}/modules | head -n1)
  EOT

  echo "%{cmdline}" > config

  %{initial-commands}

  cat > %{sysroot}%{libdir}/tmpfiles.d/extra-etc.conf << "EOF"
    C /etc/ModemManager
    C /etc/NetworkManager
    C /etc/UPower
    C /etc/X11
    C /etc/alsa
    C /etc/anthy-conf
    C /etc/appstream.conf
    C /etc/audit
    C /etc/avahi
    C /etc/bash_completion.d
    C /etc/bashrc
    C /etc/bindresvport.blacklist
    C /etc/binfmt.d
    C /etc/cups
    C /etc/cupshelpers
    C /etc/dbus-1
    C /etc/dconf
    C /etc/debuginfod
    C /etc/default
    C /etc/dracut.conf.d
    C /etc/e2scrub.conf
    C /etc/environment
    C /etc/ethertypes
    C /etc/fish
    C /etc/flatpak
    C /etc/fonts
    C /etc/fstab
    C /etc/fuse.conf
    C /etc/fwupd
    C /etc/gamemode.ini
    C /etc/gdm
    C /etc/geoclue
    C /etc/grub.d
    C /etc/gss
    C /etc/gtk-3.0
    C /etc/hosts
    C /etc/init.d
    C /etc/iscsi
    C /etc/isns
    C /etc/kernel
    C /etc/ld.so.conf
    C /etc/libaudit.conf
    C /etc/libblockdev
    C /etc/libinput
    C /etc/libnl
    C /etc/libsmbios
    C /etc/lightdm
    C /etc/locale.conf
    C /etc/localtime
    C /etc/login.defs
    C /etc/logrotate.d
    C /etc/lvm
    C /etc/man_db.conf
    C /etc/mke2fs.conf
    C /etc/modules-load.d
    C /etc/moduli
    C /etc/netconfig
    C /etc/nscd.conf
    C /etc/openfortivpn
    C /etc/openldap
    C /etc/opensc.conf
    C /etc/passwd
    C /etc/pkcs11
    C /etc/pki
    C /etc/plymouth
    C /etc/polkit-1
    C /etc/profile
    C /etc/profile.d
    C /etc/protocols
    C /etc/pulse
    C /etc/rc_keymaps
    C /etc/rc_maps.cfg
    C /etc/rpc
    C /etc/rygel.conf
    C /etc/samba
    C /etc/sane.d
    C /etc/securetty
    C /etc/security
    C /etc/services
    C /etc/shells
    C /etc/skel
    C /etc/speech-dispatcher
    C /etc/ssh_config
    C /etc/sshd_config
    C /etc/ssl
    C /etc/subgid
    C /etc/subuid
    C /etc/sudo.conf
    C /etc/sudo_logsrvd.conf
    C /etc/sudoers
    C /etc/sudoers.d
    C /etc/sudoers.dist
    C /etc/sysctl.d
    C /etc/tpm2-tss
    C /etc/udev
    C /etc/udisks2
    C /etc/uresourced.conf
    C /etc/vdpau_wrapper.cfg
    C /etc/xattr.conf
    C /etc/xdg
    C /etc/xml
    C /etc/xtables.conf
    C! /etc/audit
    C! /etc/pam.d
    C! /etc/security
    C! /etc/subgid
    C! /etc/subuid
    C! /etc/systemd/coredump.conf
    C! /etc/systemd/homed.conf
    C! /etc/systemd/journal-upload.conf
    C! /etc/systemd/journald.conf
    C! /etc/systemd/logind.conf
    C! /etc/systemd/networkd.conf
    C! /etc/systemd/oomd.conf
    C! /etc/systemd/pstore.conf
    C! /etc/systemd/resolved.conf
    C! /etc/systemd/sleep.conf
    C! /etc/systemd/system.conf
    C! /etc/systemd/timesyncd.conf
    C! /etc/systemd/user.conf
    C! /etc/issue.net
  EOF

  [[ -d %{sysroot}/%{datadir}/factory/etc ]] && rm -rf %{sysroot}/%{datadir}/factory/etc
  mkdir -p %{sysroot}/%{datadir}/factory
  mv -T %{sysroot}/etc %{sysroot}/%{datadir}/factory/etc

  cp %{sysroot}/%{datadir}/factory/etc/os-release %{sysroot}/%{libdir}/
  echo "IMAGE_VERSION=%{version}" >> %{sysroot}/%{libdir}/os-release
  echo "VARIANT=%{variant}" >> %{sysroot}/%{libdir}/os-release

  mkdir -p "%{sysroot}/%{libdir}/systemd/system/systemd-firstboot.service.d/"
  cat <<EOF >"%{sysroot}/%{libdir}/systemd/system/systemd-firstboot.service.d/no-prompt.conf"
  [Service]
  ExecStart=
  ExecStart=/usr/bin/systemd-firstboot --root-password=root
  StandardOutput=journal
  StandardInput=null
  StandardError=journal
  EOF

  mkdir -p "%{sysroot}/%{libdir}/systemd/system/ldconfig.service.d/"
  cat <<EOF >"%{sysroot}/%{libdir}/systemd/system/ldconfig.service.d/after-tmpfiles.conf"
  [Unit]
  After=systemd-tmpfiles-setup.service
  EOF
  
  mkdir -p %{install-root}
  mksquashfs %{sysroot}/usr %{install-root}/sysroot.img -noappend

build-depends:
  - external/cryptsetup
  - external/squashfs-tools
  - external/systemd
  - external/libnvme