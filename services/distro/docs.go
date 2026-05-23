/*
`distro` is the backend service for the `cmd/distro` tool.

Responsibilities:
- Manages distro image list/pull/run/remove operations.
- Runs isolated distro execution and shell sessions.
- Provides Waylayer-backed Wayland bridge for GUI apps in distro sessions.

Operational note:
  - End users typically interact through `distro` command, not this daemon
    directly.
*/
package main
