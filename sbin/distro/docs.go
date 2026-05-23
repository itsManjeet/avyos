/*
`distro` manages the optional Linux distro compatibility layer in avyos.

You can use it to download a distro root filesystem, run commands inside it,
open an interactive shell, and remove installed distros.

## Supported distros (default registry)
- `alpine`
- `debian`
- `ubuntu`
- `arch`

## Stability note
- Alpine is currently the most tested distro in this workflow.

## Configuration (`/etc/distro.ini`)
- Registry config lives at `/etc/distro.ini`.
- Each distro is configured in its own section (for example `[alpine]`).
- Common keys: `enabled`, `version`, and `url`.
- Architecture keys: `url.<goarch>`, `arch`, and `arch.<goarch>`.
  - `<arch>` uses mapped distro arch token (for example `x86_64` or `aarch64`).
  - `<goarch>` uses runtime Go arch (for example `amd64` or `arm64`).

Example config section:
```ini
[alpine]
enabled = true
version = 3.20
arch.amd64 = x86_64
arch.arm64 = aarch64
url = https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/<arch>/alpine-minirootfs-3.20.3-<arch>.tar.gz
```

## Wayland GUI support
- GUI apps inside the distro use Waylayer for Wayland compatibility bridging.
- Make sure your user session has Waylayer available.
- Launch GUI apps from distro environments only after Waylayer is up.
- Wayland support through this distro layer is very experimental.

## Example usage
```sh
# Show available registry entries
distro list --available

# Install Alpine
distro pull alpine

# Run a non-interactive command
distro run alpine cat /etc/os-release

# Run with a bind mount and working directory
distro run --bind /users:/users --workdir /users/root alpine ls -la

# Open an interactive shell
distro shell alpine

# Remove an installed distro
distro remove alpine --force
```
*/
package main
