/*
`delete` removes files or directories.

Use `--recursive` for directories and `--force` to skip prompts/overwrite checks.

Example usage:
```sh
delete old.log
delete --recursive cache/
delete --force --recursive /tmp/test-data
```

Caution:
- Deletion is destructive.
*/
package main
