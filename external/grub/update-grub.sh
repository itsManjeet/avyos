#!/bin/sh

if [[ "$EUID" -ne 0 ]] ; then 
    echo "Error! need superuser access"
    exit
fi

GRUB_DEVICE=$(mount | grep " /boot " | awk '{print $1}')    \
GRUB_DEVICE_BOOT=${GRUB_DEVICE}                             \
grub-mkconfig -o /boot/grub/grub.cfg
