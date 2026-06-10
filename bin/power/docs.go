/*
`power` performs power and session actions.

Main subcommands:
- `off` shuts down the system.
- `reboot` restarts the system.
- `logout` ends the active graphical session.

Example usage:
```sh
power off
power reboot
power logout
```

Caution:
  - `--force` bypasses service-managed shutdown/reboot and should be used only
    for emergency recovery.
*/
package main
