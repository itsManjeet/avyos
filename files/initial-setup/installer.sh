#!/bin/bash

set -euo pipefail

UUID_NAMESPACE="df2427db-01ec-4c99-96b1-be3edb3cd9f6"
ESP_TYPE_GUID="c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
USR_TYPE_GUID="8484680c-9521-48c6-9c11-b0720656f69e"
USR_VERITY_TYPE_GUID="77ff5f63-e7b6-4633-acf4-1565b864c0e6"
ROOT_TYPE_GUID="4f68bce3-e8cd-4db1-96e7-fbcaf984b709"

ESP_SIZE_MIB=500
USR_SIZE_MIB=3072
USR_VERITY_SIZE_MIB=32
ROOT_MIN_SIZE_MIB=2048
FIRST_PARTITION_MIB=1

# Inactive update slot. Keep it the same size as slot A unless overridden.
USR_B_SIZE_MIB="${ISE_USR_B_SIZE_MIB:-$USR_SIZE_MIB}"
USR_VERITY_B_SIZE_MIB="${ISE_USR_VERITY_B_SIZE_MIB:-$USR_VERITY_SIZE_MIB}"

MOUNTED=()
TEMP_DIRS=()
REPART_BIN=""

MiB=$((1024 * 1024))

die() {
    echo "installer: $*" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

find_systemd_repart() {
    if command -v systemd-repart >/dev/null 2>&1; then
        command -v systemd-repart
        return 0
    fi

    if [[ -x /usr/lib/systemd/systemd-repart ]]; then
        echo /usr/lib/systemd/systemd-repart
        return 0
    fi

    die "required command not found: systemd-repart"
}

run() {
    if (( EUID == 0 )); then
        "$@"
    else
        sudo "$@"
    fi
}

cleanup() {
    local mountpoint dir

    for ((i = ${#MOUNTED[@]} - 1; i >= 0; i--)); do
        mountpoint="${MOUNTED[$i]}"
        run umount "$mountpoint" 2>/dev/null || true
    done

    for dir in "${TEMP_DIRS[@]}"; do
        rm -rf -- "$dir" 2>/dev/null || true
    done
}
trap cleanup EXIT

lower() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

hex_to_uuid() {
    local hex

    hex="$(lower "$1")"
    [[ "$hex" =~ ^[0-9a-f]{32}$ ]] || die "invalid UUID hex: $1"
    printf '%s-%s-%s-%s-%s\n' \
        "${hex:0:8}" "${hex:8:4}" "${hex:12:4}" "${hex:16:4}" "${hex:20:12}"
}

stable_uuid() {
    uuidgen -s --namespace "$UUID_NAMESPACE" --name "$1"
}

mib_to_bytes() {
    local mib="$1"
    [[ "$mib" =~ ^[0-9]+$ ]] || die "invalid MiB value: $mib"
    echo $((mib * MiB))
}

bytes_to_mib_round_up() {
    local bytes="$1"
    echo $(((bytes + MiB - 1) / MiB))
}

resolve_block_device() {
    local device="$1"

    [[ -b "$device" ]] || die "not a block device: $device"
    readlink -f "$device"
}

block_type() {
    lsblk -dn -o TYPE "$1" 2>/dev/null | head -n1
}

require_disk() {
    local disk="$1"

    [[ "$(block_type "$disk")" == "disk" ]] ||
        die "ISE_DEVICE must be a whole disk, not a partition: $disk"
}

parent_disk_for() {
    local device="$1"
    local pkname type

    type="$(block_type "$device")"
    if [[ "$type" == "disk" ]]; then
        echo "$device"
        return 0
    fi

    pkname="$(lsblk -dn -o PKNAME "$device" 2>/dev/null | head -n1)"
    [[ -n "$pkname" ]] || return 1
    echo "/dev/${pkname}"
}

find_payload_dir() {
    local script_dir candidate

    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

    for candidate in \
        "${ISE_PAYLOAD_DIR:-}" \
        "${ISE_INSTALL_PAYLOAD:-}" \
        "$script_dir" \
        "$script_dir/../install" \
        "/run/initramfs/avyos-live/install" \
        "/run/avyos-live-iso/install" \
        "/mnt/install"; do
        [[ -n "$candidate" ]] || continue
        if [[ -d "$candidate/boot" &&
              -e "$candidate/usr.squashfs" &&
              -e "$candidate/usr.verity" &&
              -e "$candidate/usr-root-hash.txt" ]]; then
            cd "$candidate"
            pwd -P
            return 0
        fi
    done

    die "could not find installer payload; set ISE_PAYLOAD_DIR"
}

payload_file() {
    local path="$1"
    local resolved

    resolved="$(readlink -f "$path")"
    [[ -f "$resolved" ]] || die "missing payload file: $path"
    echo "$resolved"
}

live_media_disk() {
    local source

    source="$(findmnt -n -o SOURCE /run/initramfs/avyos-live 2>/dev/null || true)"
    [[ "$source" == /dev/* ]] || return 1
    parent_disk_for "$(resolve_block_device "$source")"
}

unmount_target_partitions() {
    local disk="$1"
    local mountpoint

    while read -r mountpoint; do
        [[ -n "$mountpoint" ]] || continue
        echo ":: Unmounting ${mountpoint}"
        run umount "$mountpoint" || die "failed to unmount $mountpoint"
    done < <(lsblk -rno MOUNTPOINT "$disk" | awk 'NF { print }' | sort -r)
}

swapoff_target_partitions() {
    local disk="$1"
    local part

    while read -r part; do
        [[ -n "$part" ]] || continue
        if awk -v part="$part" '$1 == part { found = 1 } END { exit !found }' /proc/swaps; then
            echo ":: Disabling swap on ${part}"
            run swapoff "$part" || die "failed to disable swap on $part"
        fi
    done < <(lsblk -rno PATH "$disk" | tail -n +2)
}

wait_for_partition_table() {
    local disk="$1"

    run partprobe "$disk" || true
    run udevadm settle || true
}

partition_by_partlabel() {
    local disk="$1"
    local label="$2"
    local path

    path="$(lsblk -rno PATH,PARTLABEL "$disk" |
        awk -v label="$label" '$2 == label { print $1; exit }')"
    [[ -n "$path" ]] || die "could not find partition labeled ${label} on ${disk}"
    echo "$path"
}

check_payload_fits_bytes() {
    local image="$1"
    local size_mib="$2"
    local image_bytes partition_bytes

    image_bytes="$(stat -c '%s' "$image")"
    partition_bytes="$(mib_to_bytes "$size_mib")"
    (( image_bytes <= partition_bytes )) ||
        die "$(basename "$image") is ${image_bytes} bytes, larger than configured ${size_mib} MiB partition"
}

check_payload_fits() {
    local image="$1"
    local partition="$2"
    local image_bytes partition_bytes

    image_bytes="$(stat -c '%s' "$image")"
    partition_bytes="$(run blockdev --getsize64 "$partition")"
    (( image_bytes <= partition_bytes )) ||
        die "$(basename "$image") is larger than $partition"
}

check_ab_space() {
    local disk="$1"
    local target_bytes base_without_b_bytes b_bytes min_bytes available_for_b_mib required_b_mib

    target_bytes="$(run blockdev --getsize64 "$disk")"

    base_without_b_bytes=$(((FIRST_PARTITION_MIB + ESP_SIZE_MIB + USR_SIZE_MIB + USR_VERITY_SIZE_MIB + ROOT_MIN_SIZE_MIB) * MiB))
    b_bytes=$(((USR_B_SIZE_MIB + USR_VERITY_B_SIZE_MIB) * MiB))
    min_bytes=$((base_without_b_bytes + b_bytes))

    if (( target_bytes < min_bytes )); then
        required_b_mib=$((USR_B_SIZE_MIB + USR_VERITY_B_SIZE_MIB))
        if (( target_bytes > base_without_b_bytes )); then
            available_for_b_mib="$(bytes_to_mib_round_up "$((target_bytes - base_without_b_bytes))")"
        else
            available_for_b_mib=0
        fi

        die "not enough space for _b partitions: need ${required_b_mib} MiB for usr_b + usr_verity_b, only ${available_for_b_mib} MiB available after ESP, usr_a, usr_verity_a, and minimum root"
    fi
}

make_temp_dir() {
    local dir

    dir="$(mktemp -d)"
    TEMP_DIRS+=("$dir")
    echo "$dir"
}

mount_partition() {
    local part="$1"
    local where="$2"

    run mount "$part" "$where"
    MOUNTED+=("$where")
}

write_repart_conf() {
    local file="$1"
    local type_guid="$2"
    local label="$3"
    local uuid="$4"
    local size_min_mib="$5"
    local size_max_mib="${6:-}"
    local no_auto="${7:-}"
    local read_only="${8:-}"

    {
        echo '[Partition]'
        echo "Type=${type_guid}"
        echo "Label=${label}"
        echo "UUID=${uuid}"
        echo "SizeMinBytes=$(mib_to_bytes "$size_min_mib")"
        if [[ -n "$size_max_mib" ]]; then
            echo "SizeMaxBytes=$(mib_to_bytes "$size_max_mib")"
        fi
        if [[ -n "$no_auto" ]]; then
            echo "NoAuto=${no_auto}"
        fi
        if [[ -n "$read_only" ]]; then
            echo "ReadOnly=${read_only}"
        fi
    } >"$file"
}

create_repart_definitions() {
    local dir="$1"
    local efi_part_uuid="$2"
    local usr_a_part_uuid="$3"
    local usr_verity_a_part_uuid="$4"
    local usr_b_part_uuid="$5"
    local usr_verity_b_part_uuid="$6"
    local root_part_uuid="$7"

    mkdir -p "$dir"

    write_repart_conf "$dir/00-esp.conf" \
        "$ESP_TYPE_GUID" "EFI" "$efi_part_uuid" \
        "$ESP_SIZE_MIB" "$ESP_SIZE_MIB"

    write_repart_conf "$dir/10-usr-a.conf" \
        "$USR_TYPE_GUID" "avyos_usr_a" "$usr_a_part_uuid" \
        "$USR_SIZE_MIB" "$USR_SIZE_MIB"

    write_repart_conf "$dir/20-usr-verity-a.conf" \
        "$USR_VERITY_TYPE_GUID" "avyos_usr_verity_a" "$usr_verity_a_part_uuid" \
        "$USR_VERITY_SIZE_MIB" "$USR_VERITY_SIZE_MIB" "1" "1"

    # Inactive A/B update slot. It is intentionally created during install;
    # if the disk cannot fit it, check_ab_space() errors before repartitioning.
    write_repart_conf "$dir/30-usr-b.conf" \
        "$USR_TYPE_GUID" "avyos_usr_b" "$usr_b_part_uuid" \
        "$USR_B_SIZE_MIB" "$USR_B_SIZE_MIB" "1"

    write_repart_conf "$dir/40-usr-verity-b.conf" \
        "$USR_VERITY_TYPE_GUID" "avyos_usr_verity_b" "$usr_verity_b_part_uuid" \
        "$USR_VERITY_B_SIZE_MIB" "$USR_VERITY_B_SIZE_MIB" "1" "1"

    # No SizeMaxBytes here: root gets all remaining space after fixed slots.
    write_repart_conf "$dir/90-root.conf" \
        "$ROOT_TYPE_GUID" "avyos_root" "$root_part_uuid" \
        "$ROOT_MIN_SIZE_MIB"
}

run_repart() {
    local disk="$1"
    local definitions_dir="$2"

    echo ":: Creating GPT partition table on ${disk} with systemd-repart"
    run wipefs -a "$disk" || true
    run "$REPART_BIN" \
        --definitions="$definitions_dir" \
        --empty=force \
        --discard=yes \
        --dry-run=no \
        "$disk" || die "systemd-repart failed to create the AVYOS partition layout"

    wait_for_partition_table "$disk"
}

format_and_populate_esp() {
    local esp_part="$1"
    local boot_source="$2"
    local esp_mount

    echo ":: Formatting EFI system partition"
    run mkfs.vfat -F 32 -n EFI "$esp_part"

    sync
    sleep 1

    esp_mount="$(make_temp_dir)"
    mount_partition "$esp_part" "$esp_mount"

    echo ":: Installing boot files"
    run cp -a "${boot_source}/." "$esp_mount/"
    sync
}

format_and_populate_root() {
    local root_part="$1"
    local root_mount

    echo ":: Formatting root filesystem"
    run mkfs.ext4 -F -L avyos-root "$root_part" >/dev/null

    root_mount="$(make_temp_dir)"
    mount_partition "$root_part" "$root_mount"

    echo ":: Creating root filesystem skeleton"
    run install -d -m 0755 \
        "$root_mount/boot" \
        "$root_mount/dev" \
        "$root_mount/etc" \
        "$root_mount/home" \
        "$root_mount/mnt" \
        "$root_mount/opt" \
        "$root_mount/proc" \
        "$root_mount/run" \
        "$root_mount/srv" \
        "$root_mount/sys" \
        "$root_mount/usr" \
        "$root_mount/var"
    run install -d -m 0700 "$root_mount/root"
    run install -d -m 1777 "$root_mount/tmp"
    run ln -s usr/bin "$root_mount/bin"
    run ln -s usr/bin "$root_mount/sbin"
    run ln -s usr/lib "$root_mount/lib"
    run ln -s usr/lib "$root_mount/lib64"
    sync
}

write_payload_partitions() {
    local usr_image="$1"
    local usr_verity="$2"
    local usr_part="$3"
    local usr_verity_part="$4"

    check_payload_fits "$usr_image" "$usr_part"
    check_payload_fits "$usr_verity" "$usr_verity_part"

    echo ":: Writing usr image to ${usr_part}"
    run dd if="$usr_image" of="$usr_part" bs=16M status=progress conv=fsync

    echo ":: Writing usr verity data to ${usr_verity_part}"
    run dd if="$usr_verity" of="$usr_verity_part" bs=4M status=progress conv=fsync
    sync
}

if [[ "${ISE_CLEAN_INSTALL:-0}" != "1" ]]; then
    die "only full disk install is supported; set ISE_CLEAN_INSTALL=1"
fi

[[ -n "${ISE_DEVICE:-}" ]] ||
    die "installer script called without ISE_DEVICE set"

need awk
need blockdev
need cp
need dd
need findmnt
need head
need install
need lsblk
need mkdir
need mktemp
need mkfs.ext4
need mkfs.vfat
need mount
need partprobe
need readlink
need rm
need sort
need stat
need swapoff
need tail
need tr
need udevadm
need umount
need uuidgen
need wipefs
if (( EUID != 0 )); then
    need sudo
fi
REPART_BIN="$(find_systemd_repart)"

PAYLOAD_DIR="$(find_payload_dir)"
USR_IMAGE="$(payload_file "$PAYLOAD_DIR/usr.squashfs")"
USR_VERITY="$(payload_file "$PAYLOAD_DIR/usr.verity")"
USR_ROOT_HASH="$(tr -d '[:space:]' <"$PAYLOAD_DIR/usr-root-hash.txt")"

[[ "$USR_ROOT_HASH" =~ ^[0-9a-fA-F]{64,}$ ]] ||
    die "invalid usr root hash in $PAYLOAD_DIR/usr-root-hash.txt"

USR_PART_UUID="$(hex_to_uuid "${USR_ROOT_HASH:0:32}")"
USR_VERITY_PART_UUID="$(hex_to_uuid "${USR_ROOT_HASH: -32}")"

EFI_PART_UUID="$(stable_uuid partition-efi)"
ROOT_PART_UUID="$(stable_uuid partition-root)"
USR_B_PART_UUID="$(stable_uuid partition-usr-b)"
USR_VERITY_B_PART_UUID="$(stable_uuid partition-usr-verity-b)"

TARGET_DISK="$(resolve_block_device "$ISE_DEVICE")"
require_disk "$TARGET_DISK"

check_payload_fits_bytes "$USR_IMAGE" "$USR_SIZE_MIB"
check_payload_fits_bytes "$USR_VERITY" "$USR_VERITY_SIZE_MIB"
check_payload_fits_bytes "$USR_IMAGE" "$USR_B_SIZE_MIB"
check_payload_fits_bytes "$USR_VERITY" "$USR_VERITY_B_SIZE_MIB"
check_ab_space "$TARGET_DISK"

if LIVE_DISK="$(live_media_disk 2>/dev/null)"; then
    [[ "$(readlink -f "$LIVE_DISK")" != "$TARGET_DISK" ]] ||
        die "refusing to install over the live installer media: $TARGET_DISK"
fi

REPART_DIR="$(make_temp_dir)"
create_repart_definitions \
    "$REPART_DIR" \
    "$EFI_PART_UUID" \
    "$USR_PART_UUID" \
    "$USR_VERITY_PART_UUID" \
    "$USR_B_PART_UUID" \
    "$USR_VERITY_B_PART_UUID" \
    "$ROOT_PART_UUID"

echo "Target Disk: ${TARGET_DISK}"
echo "Payload: ${PAYLOAD_DIR}"
echo "Repart definitions: ${REPART_DIR}"

unmount_target_partitions "$TARGET_DISK"
swapoff_target_partitions "$TARGET_DISK"
run_repart "$TARGET_DISK" "$REPART_DIR"

sync
sleep 1

ESP_PART="$(partition_by_partlabel "$TARGET_DISK" EFI)"
USR_PART="$(partition_by_partlabel "$TARGET_DISK" avyos_usr_a)"
USR_VERITY_PART="$(partition_by_partlabel "$TARGET_DISK" avyos_usr_verity_a)"
USR_B_PART="$(partition_by_partlabel "$TARGET_DISK" avyos_usr_b)"
USR_VERITY_B_PART="$(partition_by_partlabel "$TARGET_DISK" avyos_usr_verity_b)"
ROOT_PART="$(partition_by_partlabel "$TARGET_DISK" avyos_root)"

# Keep these variables visible in logs; future updater can use labels/UUIDs too.
echo "Slot A usr: ${USR_PART}"
echo "Slot A verity: ${USR_VERITY_PART}"
echo "Slot B usr: ${USR_B_PART}"
echo "Slot B verity: ${USR_VERITY_B_PART}"

format_and_populate_esp "$ESP_PART" "$PAYLOAD_DIR/boot"
write_payload_partitions "$USR_IMAGE" "$USR_VERITY" "$USR_PART" "$USR_VERITY_PART"
format_and_populate_root "$ROOT_PART"

wait_for_partition_table "$TARGET_DISK"

echo ":: Installation complete"
sync
sleep 5
exit 0
