#!/usr/bin/env bash
set -euo pipefail

OUT=disk.img
DISK_SIZE=""
EFI_SIZE_MIB=200
BIOS_SIZE_MIB=1
MIN_USR_MIB=256
MIN_DATA_MIB=512
LIMINE_PATH=/usr/share/limine
GPT_TAIL_MIB=4
ARCH=$(uname -m)

while [[ $# -gt 0 ]]; do
    case "$1" in
        -out)         OUT="$2";          shift 2 ;;
        -arch)        ARCH="$2";         shift 2 ;;
        -disk-size)   DISK_SIZE="$2";    shift 2 ;;
        -kernel)      KERNEL="$2";       shift 2 ;;
        -initrd)      INITRD="$2";       shift 2 ;;
        -usr)         USRFS="$2";       shift 2 ;;
        -limine-path) LIMINE_PATH="$2";  shift 2 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

[[ -f "${KERNEL:-}" ]] || { echo "Missing -kernel"; exit 1; }
[[ -f "${INITRD:-}" ]] || { echo "Missing -initrd"; exit 1; }
[[ -f "${USRFS:-}" ]] || { echo "Missing -usr"; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

usr_bytes=$(du -sb "$USRFS" | cut -f1)
usr_mib=$(( (usr_bytes + 1024*1024 - 1) / (1024*1024) ))

USR_SIZE_MIB=$(( usr_mib * 110 / 100 ))
(( USR_SIZE_MIB < MIN_USR_MIB )) && USR_SIZE_MIB=$MIN_USR_MIB

if [[ -z "$DISK_SIZE" ]]; then
    total_mib=$(( BIOS_SIZE_MIB + EFI_SIZE_MIB + USR_SIZE_MIB + MIN_DATA_MIB + GPT_TAIL_MIB ))
    DISK_SIZE="${total_mib}M"
fi

BIOS_START=1
BIOS_END=$(( BIOS_START + BIOS_SIZE_MIB ))

EFI_START=$BIOS_END
EFI_END=$(( EFI_START + EFI_SIZE_MIB ))

USR_START=$EFI_END
USR_END=$(( USR_START + USR_SIZE_MIB ))

DATA_START=$USR_END

echo "[*] Creating disk: $OUT ($DISK_SIZE)"
truncate -s "$DISK_SIZE" "$OUT"

parted -s "$OUT" mklabel gpt

parted -s "$OUT" mkpart bios "${BIOS_START}MiB" "${BIOS_END}MiB"
parted -s "$OUT" set 1 bios_grub on

parted -s "$OUT" mkpart esp fat32 "${EFI_START}MiB" "${EFI_END}MiB"
parted -s "$OUT" set 2 esp on

parted -s "$OUT" mkpart usr "${USR_START}MiB" "${USR_END}MiB"

DISK_BYTES=$(stat -c%s "$OUT")
DISK_MIB=$(( DISK_BYTES / 1024 / 1024 ))
DATA_END=$(( DISK_MIB - GPT_TAIL_MIB ))

if (( DATA_END <= DATA_START )); then
    echo "Error: disk too small for data partition"
    exit 1
fi

parted -s "$OUT" mkpart data "${DATA_START}MiB" "${DATA_END}MiB"

USR_PARTNUM=3
USR_PARTUUID=$(
    parted -m "$OUT" print \
    | awk -F: -v p="$USR_PARTNUM" '$1 == p { print $7 }'
)

if [[ -z "$USR_PARTUUID" || "$USR_PARTUUID" == "-" ]]; then
    echo "Failed to read PARTUUID from parted"
    exit 1
fi

USR_PARTUUID=$(echo "$USR_PARTUUID" | tr 'A-Z' 'a-z')
echo "[*] System PARTUUID: $USR_PARTUUID"

echo "[*] Building EFI filesystem"
EFI_IMG="$WORK/efi.img"
truncate -s "${EFI_SIZE_MIB}M" "$EFI_IMG"
mkfs.vfat "$EFI_IMG"

mmd   -i "$EFI_IMG" ::/EFI
mmd   -i "$EFI_IMG" ::/EFI/BOOT
mmd   -i "$EFI_IMG" ::/limine

mcopy -i "$EFI_IMG" "$KERNEL" ::/kernel
mcopy -i "$EFI_IMG" "$INITRD" ::/initrd

if [ "$ARCH" == "x86_64" ] ; then
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/BOOTX64.EFI" ::/EFI/BOOT/
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/limine-bios.sys" ::/limine
elif [ "$ARCH" == "aarch64" ] ; then
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/BOOTAA64.EFI" ::/EFI/BOOT/
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

cat > "$WORK/limine.conf" <<EOF
timeout: 5

/AvyOS
    protocol: linux
    path: boot():/kernel
    cmdline: usr=/dev/sda3 usrfstype=squashfs root=/dev/sda4 usrtype=btrfs loglevel=7 console=tty0 console=ttyS0
    module_path: boot():/initrd
EOF

mcopy -i "$EFI_IMG" "$WORK/limine.conf" ::/limine/limine.conf

dd if="$EFI_IMG" of="$OUT" bs=1M seek="$EFI_START" conv=notrunc status=none

echo "[*] Building SquashFS root"
dd if="$USRFS" of="$OUT" bs=1M seek="$USR_START" conv=notrunc status=none

echo "[*] Building Btrfs data"
DATA_MIB=$(( DATA_END - DATA_START ))
DATA_IMG="$WORK/data.img"

truncate -s "${DATA_MIB}M" "$DATA_IMG"
mkfs.btrfs -f "$DATA_IMG"

dd if="$DATA_IMG" of="$OUT" bs=1M seek="$DATA_START" conv=notrunc status=none

if [ "$ARCH" == "x86_64" ] ; then
    limine bios-install "$OUT" 1
fi

echo "[✓] Image ready: $OUT"
