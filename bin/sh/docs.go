/*
`sh` is the interactive avyos command shell.

Capabilities:
- Runs built-in and external commands.
- Supports aliases, history, and environment variables.
- Supports pipes and basic redirection (`|`, `>`, `>>`, `<`).

Built-in examples:
```sh
help
cd /users/alice
alias ll='list --long'
export EDITOR=notepad
history
```
*/
package main
