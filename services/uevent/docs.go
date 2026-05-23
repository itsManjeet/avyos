/*
`uevent` is the device-event monitoring and catalog service.

Responsibilities:
- Tracks hardware add/remove/change events.
- Maintains device metadata for query APIs.
- Serves device information to `cmd/uevent` and other clients.

Operational note:
- It is a background system service started during boot.
*/
package main
