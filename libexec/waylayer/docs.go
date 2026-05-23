/*
`waylayer` exposes a Wayland socket for Linux applications and adapts their
windows to the AvyOS desktop service.

Supported protocol surface:
- wl_compositor and wl_shm with ARGB8888/XRGB8888 buffers.
- xdg-shell toplevel windows and server-side decoration negotiation.
- Pointer and keyboard input through wl_seat.
- Frame callbacks, damage, resize configuration, and graceful close requests.

The default socket is `/run/user/<uid>/dev.avyos.waylayer`. The distro service relays its
container-side `WAYLAND_DISPLAY=waylayer` socket to this socket while preserving
SCM_RIGHTS file descriptors.
*/
package main
