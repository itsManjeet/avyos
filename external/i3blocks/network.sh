#!/bin/sh

network=$(ip route get 1.1.1.1 | sed -n 's/.*dev \([a-zA-Z0-9_]*\).*/\1/p')
[ -n "$network" ] && ping=$(ping -c 1 www.google.es | tail -1| awk '{print $4}' | cut -d '/' -f 2 | cut -d '.' -f 1)

if ! [ $network ] ; then
    network_active="󰲛"
else
    network_active="󰛳"
fi

echo "$network_active $interface_easyname"