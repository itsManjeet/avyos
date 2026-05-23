/*
`service` manages init services.

Main subcommands:
- `list` and `status` for inspection.
- `start`, `stop`, `restart` for control.

Example usage:
```sh
service list
service status display
service restart login
service start uevent display login
```
*/
package main
