/*
`read` reads and inspects text files.

Main modes:
- Plain read: `read <file>`
- Head/tail: `read top`, `read last`
- Search: `read pattern`
- Counters: `read count`

Example usage:
```sh
read /var/log/system.log
read top /var/log/system.log --lines=20
read last /var/log/system.log --lines=50 --follow
read pattern error /var/log/system.log --ignorecase --numbers
read count /var/log/system.log
```
*/
package main
