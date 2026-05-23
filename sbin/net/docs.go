/*
`net` provides network interface and routing utilities.

Main subcommands:
- `interfaces` and `address` for inspection.
- `start` to bring an interface up.
- `assign` to set an IPv4 CIDR address.
- `route` to set default gateway route.

Example usage:
```sh
net interfaces
net address eth0
net start eth0
net assign eth0 192.168.1.20/24
net route eth0 192.168.1.1
```
*/
package main
