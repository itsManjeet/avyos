/*
`request` provides connectivity and network request tools.

Main subcommands:
- `ping` connectivity probe.
- `fetch` HTTP/HTTPS download.
- `listen` TCP server socket.
- `connect` TCP client session.
- `dns` DNS record lookup.

Example usage:
```sh
request ping avyos.dev --count=3
request fetch https://example.com --output page.html
request dns avyos.dev
request listen 8080
request connect 127.0.0.1:8080
```
*/
package main
