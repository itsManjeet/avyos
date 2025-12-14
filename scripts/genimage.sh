#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Defaults
# ============================================================
OUT=avyos.img
DISK_SIZE=""
EFI_SIZE_MIB=200
BIOS_SIZE_MIB=1
MIN_SYS_MIB=256
MIN_DATA_MIB=512
PROTOCOL=linux
LIMINE_PATH=/usr/share/limine
GPT_TAIL_MIB=4

# ============================================================
# Args
# ============================================================
while [[ $# -gt 0 ]]; do
    case "$1" in
        -out)         OUT="$2";          shift 2 ;;
        -disk-size)   DISK_SIZE="$2";    shift 2 ;;
        -kernel)      KERNEL="$2";       shift 2 ;;
        -initrd)      INITRD="$2";       shift 2 ;;
        -rootfs)      ROOTFS="$2";       shift 2 ;;
        -protocol)    PROTOCOL="$2";     shift 2 ;;
        -limine-path) LIMINE_PATH="$2";  shift 2 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

[[ -f "${KERNEL:-}" ]] || { echo "Missing -kernel"; exit 1; }
[[ -f "${INITRD:-}" ]] || { echo "Missing -initrd"; exit 1; }
[[ -d "${ROOTFS:-}" ]] || { echo "Missing -rootfs"; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# ============================================================
# Size calculations
# ============================================================
rootfs_bytes=$(du -sb "$ROOTFS" | cut -f1)
rootfs_mib=$(( (rootfs_bytes + 1024*1024 - 1) / (1024*1024) ))

SYS_SIZE_MIB=$(( rootfs_mib * 110 / 100 ))
(( SYS_SIZE_MIB < MIN_SYS_MIB )) && SYS_SIZE_MIB=$MIN_SYS_MIB

if [[ -z "$DISK_SIZE" ]]; then
    total_mib=$(( BIOS_SIZE_MIB + EFI_SIZE_MIB + SYS_SIZE_MIB + MIN_DATA_MIB + GPT_TAIL_MIB ))
    DISK_SIZE="${total_mib}M"
fi

# ============================================================
# Partition layout
# ============================================================
BIOS_START=1
BIOS_END=$(( BIOS_START + BIOS_SIZE_MIB ))

EFI_START=$BIOS_END
EFI_END=$(( EFI_START + EFI_SIZE_MIB ))

SYS_START=$EFI_END
SYS_END=$(( SYS_START + SYS_SIZE_MIB ))

DATA_START=$SYS_END

# ============================================================
# Create disk + GPT
# ============================================================
echo "[*] Creating disk: $OUT ($DISK_SIZE)"
truncate -s "$DISK_SIZE" "$OUT"

parted -s "$OUT" mklabel gpt

parted -s "$OUT" mkpart bios "${BIOS_START}MiB" "${BIOS_END}MiB"
parted -s "$OUT" set 1 bios_grub on

parted -s "$OUT" mkpart esp fat32 "${EFI_START}MiB" "${EFI_END}MiB"
parted -s "$OUT" set 2 esp on

parted -s "$OUT" mkpart system "${SYS_START}MiB" "${SYS_END}MiB"

DISK_BYTES=$(stat -c%s "$OUT")
DISK_MIB=$(( DISK_BYTES / 1024 / 1024 ))
DATA_END=$(( DISK_MIB - GPT_TAIL_MIB ))

if (( DATA_END <= DATA_START )); then
    echo "Error: disk too small for data partition"
    exit 1
fi

parted -s "$OUT" mkpart data "${DATA_START}MiB" "${DATA_END}MiB"

# ============================================================
# Identifing Partition UUID
# ============================================================
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

# ============================================================
# EFI filesystem (userspace)
# ============================================================
echo "[*] Building EFI filesystem"
EFI_IMG="$WORK/efi.img"
truncate -s "${EFI_SIZE_MIB}M" "$EFI_IMG"
mkfs.vfat "$EFI_IMG"

mmd   -i "$EFI_IMG" ::/EFI
mmd   -i "$EFI_IMG" ::/EFI/BOOT
mmd   -i "$EFI_IMG" ::/limine

mcopy -i "$EFI_IMG" "$KERNEL" ::/kernel
mcopy -i "$EFI_IMG" "$INITRD" ::/initrd
mcopy -i "$EFI_IMG" "$LIMINE_PATH/BOOTX64.EFI" ::/EFI/BOOT/
mcopy -i "$EFI_IMG" "$LIMINE_PATH/limine-bios.sys" ::/limine

cat > "$WORK/limine.conf" <<EOF
timeout: 5

/AvyOS
    protocol: ${PROTOCOL}
    path: boot():/kernel
    cmdline: root=/dev/sda3 rootfstype=squashfs loglevel=7 console=ttyS0 console=tty0 
    module_path: boot():/initrd
EOF

mcopy -i "$EFI_IMG" "$WORK/limine.conf" ::/limine/limine.conf

dd if="$EFI_IMG" of="$OUT" bs=1M seek="$EFI_START" conv=notrunc status=none

# ============================================================
# SquashFS system
# ============================================================
echo "[*] Building SquashFS root"
SYS_IMG="$WORK/system.squashfs"
mksquashfs "$ROOTFS" "$SYS_IMG" -noappend -comp zstd

dd if="$SYS_IMG" of="$OUT" bs=1M seek="$SYS_START" conv=notrunc status=none

# ============================================================
# Btrfs data
# ============================================================
echo "[*] Building Btrfs data"
DATA_MIB=$(( DATA_END - DATA_START ))
DATA_IMG="$WORK/data.img"

truncate -s "${DATA_MIB}M" "$DATA_IMG"
mkfs.btrfs -f "$DATA_IMG"

dd if="$DATA_IMG" of="$OUT" bs=1M seek="$DATA_START" conv=notrunc status=none

# ============================================================
# Limine BIOS install
# ============================================================
limine bios-install "$OUT" 1

echo "[✓] Image ready: $OUT"
