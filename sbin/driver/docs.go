/*
`driver` manages kernel modules.

Main subcommands:
- `list` shows loaded modules.
- `load` loads a module file or name.
- `unload` removes a loaded module.
- `info` and `deps` show module details.

Example usage:
```sh
driver list
driver load e1000e
driver info e1000e
driver deps e1000e
driver unload e1000e
```
*/
package main
