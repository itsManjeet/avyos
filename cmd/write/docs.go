/*
`write` prints formatted text to stdout, stderr, or a file.

Useful options:
- `--out` chooses destination.
- `--append` appends to files.
- `--newline` controls trailing newline.

Example usage:
```sh
write "Hello %s" avyos
write "error: %s" timeout --out=stderr
write "uptime=%d" 42 --out=/tmp/status.txt --append
```
*/
package main
