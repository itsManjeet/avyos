/*
`copy` copies files and directories.

Use it when you want to duplicate data to a new path.

Example usage:
```sh
copy notes.txt backup/notes.txt
copy --recursive projects/ backup/projects/
copy --force build/output.bin /tmp/output.bin
```
*/
package main
