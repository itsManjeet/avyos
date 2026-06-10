/*
`mount` manages filesystem mounts.

Main subcommands:
- `list` shows mounted filesystems.
- `add` mounts a device on a mount point.
- `remove` unmounts a mount point.

Example usage:
```sh
mount list
mount add /dev/sda1 /mnt/data --type ext4
mount remove /mnt/data
```

Note:
- Mount operations usually require elevated privileges.
*/
package main
