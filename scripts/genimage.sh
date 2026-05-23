#!/usr/bin/env bash
set -euo pipefail

OUT=disk.img
DISK_SIZE="5G"
EFI_SIZE_MIB=200
BIOS_SIZE_MIB=1
MIN_SYS_MIB=256
LIMINE_PATH=/usr/share/limine
GPT_TAIL_MIB=4
KARGS=""
ARCH=$(go env GOARCH)

while [[ $# -gt 0 ]]; do
    case "$1" in
        -out)         OUT="$2";          shift 2 ;;
        -arch)        ARCH="$2";         shift 2 ;;
        -disk-size)   DISK_SIZE="$2";    shift 2 ;;
        -kernel)      KERNEL="$2";       shift 2 ;;
        -initrd)      INITRD="$2";       shift 2 ;;
        -system)      SYSTEM="$2";        shift 2 ;;
        -limine-path) LIMINE_PATH="$2";  shift 2 ;;
        -kargs)       KARGS="$2";        shift 2 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

[[ -f "${KERNEL:-}" ]] || { echo "Missing -kernel"; exit 1; }
[[ -f "${INITRD:-}" ]] || { echo "Missing -initrd"; exit 1; }
[[ -f "${SYSTEM:-}" ]]  || { echo "Missing -system"; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

sys_bytes=$(du -sb "$SYSTEM" | cut -f1)
sys_mib=$(( (sys_bytes + 1024*1024 - 1) / (1024*1024) ))

SYS_SIZE_MIB=$(( sys_mib * 110 / 100 ))
(( SYS_SIZE_MIB < MIN_SYS_MIB )) && SYS_SIZE_MIB=$MIN_SYS_MIB

BIOS_START=1
BIOS_END=$(( BIOS_START + BIOS_SIZE_MIB ))

EFI_START=$BIOS_END
EFI_END=$(( EFI_START + EFI_SIZE_MIB ))

SYS_START=$EFI_END
SYS_END=$(( SYS_START + SYS_SIZE_MIB ))

echo "[*] Creating disk: $OUT ($DISK_SIZE)"
truncate -s "$DISK_SIZE" "$OUT"

parted -s "$OUT" mklabel gpt

parted -s "$OUT" mkpart bios "${BIOS_START}MiB" "${BIOS_END}MiB"
parted -s "$OUT" set 1 bios_grub on

parted -s "$OUT" mkpart esp fat32 "${EFI_START}MiB" "${EFI_END}MiB"
parted -s "$OUT" set 2 esp on

parted -s "$OUT" mkpart sys "${SYS_START}MiB" "${SYS_END}MiB"

DISK_BYTES=$(stat -c%s "$OUT")
DISK_MIB=$(( DISK_BYTES / 1024 / 1024 ))

SYS_PARTNUM=3
SYS_PARTUUID=$(
    parted -m "$OUT" print \
    | awk -F: -v p="$SYS_PARTNUM" '$1 == p { print $7 }'
)

if [[ -z "$SYS_PARTUUID" || "$SYS_PARTUUID" == "-" ]]; then
    echo "Failed to read PARTUUID from parted"
    exit 1
fi

SYS_PARTUUID=$(echo "$SYS_PARTUUID" | tr 'A-Z' 'a-z')
echo "[*] System PARTUUID: $SYS_PARTUUID"

echo "[*] Building EFI filesystem"
EFI_IMG="$WORK/efi.img"
truncate -s "${EFI_SIZE_MIB}M" "$EFI_IMG"
mkfs.vfat "$EFI_IMG"

mmd   -i "$EFI_IMG" ::/EFI
mmd   -i "$EFI_IMG" ::/EFI/BOOT
mmd   -i "$EFI_IMG" ::/limine

mcopy -i "$EFI_IMG" "$KERNEL" ::/kernel
mcopy -i "$EFI_IMG" "$INITRD" ::/initrd

if [ "$ARCH" == "amd64" ] ; then
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/BOOTX64.EFI" ::/EFI/BOOT/
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/limine-bios.sys" ::/limine
elif [ "$ARCH" == "arm64" ] ; then
    mcopy -i "$EFI_IMG" "$LIMINE_PATH/BOOTAA64.EFI" ::/EFI/BOOT/
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

cat > "$WORK/limine.conf" <<EOF
timeout: 1

/AvyOS
    protocol: linux
    path: boot():/kernel
    cmdline: usr=/dev/sda3 usrfstype=squashfs loglevel=7 ${KARGS}
    module_path: boot():/initrd
EOF

mcopy -i "$EFI_IMG" "$WORK/limine.conf" ::/limine/limine.conf

dd if="$EFI_IMG" of="$OUT" bs=1M seek="$EFI_START" conv=notrunc status=none

echo "[*] Building SquashFS root"
dd if="$SYSTEM" of="$OUT" bs=1M seek="$SYS_START" conv=notrunc status=none

if [ "$ARCH" == "amd64" ] ; then
    limine bios-install "$OUT" 1
fi

echo "[✓] Image ready: $OUT"
