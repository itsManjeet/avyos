/*
`init` is the avyos init process (PID 1).

It is responsible for early boot setup, service startup supervision,
child-process reaping, and shutdown handling.

Operational note:
- This command is started by the system during boot.
- It is not intended as a regular interactive end-user command.
*/
package main
