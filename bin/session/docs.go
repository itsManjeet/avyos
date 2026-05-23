/*
`session` starts and supervises the desktop user session.

What it starts:
- Settings daemon (optional)
- Waylayer daemon (optional)
- Background and Dock as critical desktop processes

Operational note:
- This command is usually started by login/session flow.
- If a critical desktop component exits, the session ends.
*/
package main
