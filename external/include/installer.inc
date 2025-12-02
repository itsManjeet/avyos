force-rebuild: true

pre-script: |-
    mkdir -p ISO/{boot/grub/{fonts,x86_64-efi},efi/boot,isolinux,LiveOS}

    install -vDm0644 /dev/stdin ISO/isolinux/isolinux.cfg << EOF
    DEFAULT AVYOS GNU/Linux
        LABEL AVYOS GNU/Linux
        KERNEL __KERNEL__
        APPEND initrd=__INITRD__ __KARGS__
    EOF

    install -vDm0644 /dev/stdin ISO/boot/grub/grub.cfg << EOF
    set default="AVYOS GNU/Linux"
    set timeout=10

    insmod all_video
    if loadfont /boot/grub/fonts/unicode.pf2; then
        set gfxmode=800x600
        insmod efi_gop
        insmod efi_uga
        insmod video_bochs
        insmod video_cirrus
        insmod gfxterm
        insmod png
        terminal_output gfxterm
    fi

    menuentry "AVYOS GNU/Linux" {
        linux __KERNEL__ __KARGS__
        initrd __INITRD__
    }

    EOF


post-script: |-
    for file in chain.c32 isolinux.bin ldlinux.c32 libutil.c32 reboot.c32 vesamenu.c32 libcom32.c32 poweroff.c32 ; do
        cp %{datadir}/syslinux/$file ISO/isolinux/
    done

    cp %{datadir}/grub/unicode.pf2 ISO/boot/grub/fonts/
    echo "set prefix=/boot/grub" > ISO/boot/grub-early.cfg
    cp -a %{libdir}/grub/x86_64-efi/*.{mod,lst} ISO/boot/grub/x86_64-efi

    grub-mkimage -c ISO/boot/grub-early.cfg -o ISO/efi/boot/bootx64.efi -Ox86_64-efi -p "" iso9660 normal search search_fs_file
    dd if=/dev/zero of=ISO/boot/efiboot.img count=4096
    mkdosfs -n AVYOS-UEFI ISO/boot/efiboot.img
    mmd -i ISO/boot/efiboot.img ::/EFI
    mmd -i ISO/boot/efiboot.img ::/EFI/BOOT 
    mcopy -i ISO/boot/efiboot.img ISO/efi/boot/bootx64.efi ::/EFI/BOOT

    # Dracut otherwise fail to detect OS!!
    mkdir -p %{sysroot}/proc

    mksquashfs %{sysroot} ISO/LiveOS/squashfs.img -noappend

    mkdir -p %{install-root}/
    xorriso -as mkisofs \
        -isohybrid-mbr %{datadir}/syslinux/isohdpfx.bin \
        -c isolinux/boot.cat \
        -b isolinux/isolinux.bin \
            -no-emul-boot \
            -boot-load-size 4 \
            -boot-info-table \
        -eltorito-alt-boot \
        -e boot/efiboot.img \
            -no-emul-boot \
            -isohybrid-gpt-basdat \
            -volid %{installer-volume-id} \
        -o %{install-root}/avyos-%{channel}-installer.iso ISO/

    cd %{install-root}
    sha256sum avyos-%{channel}-installer.iso >avyos-%{channel}-installer.iso.sha265sum


build-depends:
  - components/e2fsprogs.yml
  - components/dracut.yml
  - components/mtools.yml
  - components/grub.yml
  - components/systemd.yml
  - components/cryptsetup.yml
  - components/squashfs-tools.yml
  - components/syslinux.yml
  - components/plymouth.yml
  - components/genimage.yml