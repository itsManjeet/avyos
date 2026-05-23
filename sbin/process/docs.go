/*
`process` inspects and manages running processes.

Main subcommands:
- `list` to view running tasks.
- `info` for detailed process data.
- `kill` to send signals.
- `tree` to view parent/child structure.

Example usage:
```sh
process list
process info 1234
process kill --signal TERM 1234
process tree
```
*/
package main
