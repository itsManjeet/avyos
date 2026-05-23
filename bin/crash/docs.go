/*
`crash` intentionally crashes the current process for testing.

It is useful for validating the kernel coredump helper and desktop crash
notifications.

Example usage:
```sh
ulimit -c unlimited
crash segv
crash abrt
crash panic
```
*/
package main
