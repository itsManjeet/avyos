/*
`filter` searches text content in files (grep-like behavior).

Useful options include:
- `--ignore-case`, `--line-number`, `--count`
- `--recursive`, `--files-only`, `--fixed`
- `--before`, `--after`, `--context`

Example usage:
```sh
filter error app.log
filter --ignore-case --line-number timeout logs/*.log
filter --recursive --files-only panic ./services
filter --fixed --count "request id" api.log
```
*/
package main
