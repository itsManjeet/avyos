#!/bin/sh -eu

if ! lsmod | grep i2c_dev >/dev/null 2>&1; then
	echo "kernel module i2c-dev must be loaded"
	exit
fi

if ! command -v i2cdump >/dev/null 2>&1; then
	echo "i2cdump from i2c-tools must be installed"
	exit
fi


for connector in $(find /sys/class/drm/ -maxdepth 1 -name 'card[0-9]-*'); do
	bus=$(find "$connector/" -maxdepth 1 -mindepth 1 -type d -name 'i2c-*' -print -quit | sed -n 's/.*\/i2c\-\(.*\)/\1/p')
	if [ "$bus" = "" ]; then
		bus=$(find "$connector/ddc/i2c-dev/" -maxdepth 1 -mindepth 1 -type d -name 'i2c-*' -print -quit | sed -n 's/.*\/i2c\-\(.*\)/\1/p')
	fi
	if [ "$bus" = "" ]; then
		echo "Connector $(basename $connector) does not have an i2c bus. Skipping."
		continue
	fi

	echo "Dumping connector $(basename $connector) i2c bus address 0x50"
	i2cdump -y $bus 0x50 b

	echo "Dumping connector $(basename $connector) i2c bus address 0x51"
	i2cdump -y $bus 0x51 b

	echo "Dumping connector $(basename $connector) i2c bus address 0x54"
	i2cdump -y $bus 0x54 b

	echo "Dumping connector $(basename $connector) i2c bus address 0x55"
	i2cdump -y $bus 0x55 b
done

