/*
`uevent` manages device-event inspection and monitoring.

Main subcommands:
- `list` known devices.
- `info` detailed device data.
- `monitor` live add/remove/change events.
- `trigger` device rescan request.

Example usage:
```sh
uevent list
uevent info /devices/pci0000:00
uevent monitor
uevent trigger --subsystem=block
```
*/
package main
