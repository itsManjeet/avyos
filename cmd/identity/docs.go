/*
`identity` manages user identities and capabilities.

Main subcommands:
- `info`, `list`, `capabilities`
- `switch` (root required)
- `add`, `password`

Example usage:
```sh
identity info
identity list
identity add alice --kind user
identity password alice "new-password"
identity switch alice
```
*/
package main
