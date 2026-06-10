/*
`resolution` manages the system display mode used by the login and desktop shell.

Subcommands:
- `list` shows available DRM modes.
- `set <WxH[@Hz]>` saves a mode and restarts the `desktop` service.
- `clear` removes the saved override and restarts the `desktop` service.

Example usage:
```sh
resolution list
resolution set 1920x1200
resolution set 1920x1200@60
resolution clear
```
*/
package main
