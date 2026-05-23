# Building, Running, Debugging

This document describes how to build and run avyos, along with available `make` options, testing, and debugging workflows.

> For a system overview and code map, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

---

## Prerequisites

You’ll need:

- Go (version as required by `go.mod`)
- `mksquashfs` (SquashFS tools)
- `cpio`
- QEMU (for `make run`)
- Optional: `wget` (for CA bundle update)

Example (Debian/Ubuntu):

```bash
sudo apt-get update
sudo apt-get install -y golang-go squashfs-tools cpio qemu-system-x86 wget
```

---

## Quick start

Build a bootable disk image:

```bash
make
```

Run it in QEMU:

```bash
make run
```

Build artifacts are created under:

- `_cache/<arch>/disk.img` — bootable image
- `_cache/<arch>/system.img` — SquashFS OS core
- `_cache/<arch>/initramfs.img` — initramfs
- `_cache/<arch>/variables` — UEFI variables copy used by QEMU

Where `<arch>` is `GOARCH` (e.g. `amd64`, `arm64`).

---

## Make targets

### `make` / `make all`
Builds the bootable disk image.

### `make run`
Runs the built disk image in QEMU.

### `make clean`
Removes generated images and build output under `_cache`.

### `make docs`
Runs the documentation generator:

- output directory: `_cache/docs` by default (override with `DOCGEN_OUT`)

### `make compile_db`
Generates `depends.<arch>.inc` (dependency include file) used to improve incremental rebuilds.

### `make update-certificates`
Downloads a CA bundle to:

- `etc/certificates/ca-certificates.crt`

---

## Build configuration (variables)

These variables can be passed on the command line:

```bash
make <target> VAR=value
```

### Toolchain / build
- `GO`  
  Go command to use (default: `go`)

- `GOFLAGS`  
  Extra go build flags (default includes `-tags netgo`)

- `GOARCH`  
  Target architecture (default: `go env GOARCH`)

- `RELEASE=1`  
  Enables release-oriented Go flags:
  - `-buildvcs=false -trimpath -ldflags="-s -w -buildid="`

### Kernel selection
- `KERNEL=linux` *(default)*  
  Uses prebuilt kernel image:
  - `external/<arch>/kernel.img`

- `KERNEL=<non-linux>`  
  Builds the alternative kernel payload from `avyos.dev/kernel` into:
  - `_cache/<arch>/kernel.img`

  Notes:
  - kernel build uses `GOOS=linux`, `CGO_ENABLED=0`
  - `DEBUG=1` affects kernel GC/LDFLAGS (no optimizations; better for debugging)

### Cache/output paths
- `CACHE_PATH`  
  Root build cache directory (default: `<repo>/_cache`)

- `DOCGEN_OUT`  
  Output directory for docs (default: `<CACHE_PATH>/docs`)

### QEMU runtime
- `QEMU`  
  QEMU binary (auto-selected from `GOARCH`)

- `QEMU_ACCEL`  
  Acceleration (auto):
  - Linux host: `-accel kvm`
  - macOS host: `-accel hvf`

- `QEMU_VNC=1`  
  Adds VNC output: `-vnc :0`

- `DBG_PORT` *(default: 5037)*  
  Host port forwarded to the guest debug daemon (`dbgd`) port 5037.
  QEMU networking is configured as:
  - `hostfwd=tcp:127.0.0.1:<DBG_PORT>-:5037`

### Debugging
- `DEBUG=1`  
  Enables QEMU debug mode:
  - QEMU runs with `-s -S` (waits for a debugger)
  - debug logging enabled (`_cache/<arch>/qemu-debug.log`)
  - also enables kernel debug flags when using `KERNEL!=linux`

---

## Running (QEMU)

Basic run:

```bash
make run
```

With VNC:

```bash
QEMU_VNC=1 make run
```

With QEMU + kernel debug mode:

```bash
DEBUG=1 make run
```

Console selection:
- `amd64`: `console=ttyS0 console=tty0`
- `arm64`: `console=ttyAMA0,115200`

---

## Debugging

### 1) Remote debugging daemon (`dbgd`) + client (`dbg`)

avyos includes a remote debugging daemon service: **`dbgd`**.
- The daemon listens on guest TCP `:5037` by default.
- QEMU forwards that to your host at `127.0.0.1:${DBG_PORT}` (default `5037`).

The repo provides a client at `tools/dbg` (host tool) to:
- run commands remotely
- open an authenticated shell
- pull files from the guest
- push files into the guest

Build and run the host client:

```bash
go run ./tools/dbg --help
```

#### Authentication
The client authenticates against `dbgd` using:
- `--user` (default: `admin`)
- `--password` or environment variable `DBG_PASSWORD`
- if neither is provided, it will prompt

#### Common usage

Run a command (no shell parsing):

```bash
go run ./tools/dbg --host=127.0.0.1 --port=${DBG_PORT} cmd list /etc
```

Run a command using shell semantics:

```bash
go run ./tools/dbg --port=${DBG_PORT} shell "list /etc | read pattern services"
```

Download a file from the guest:

```bash
go run ./tools/dbg --port=${DBG_PORT} pull /etc/init.conf ./init.conf
```

Upload a file to the guest:

```bash
go run ./tools/dbg --port=${DBG_PORT} push ./init.conf /etc/init.conf
```

Show authenticated identity:

```bash
go run ./tools/dbg --port=${DBG_PORT} whoami
```

---

### 3) Delve in the guest (Go debugging)

The build includes Delve in the OS core at:

A typical flow:
1. boot avyos (via `make run`)
2. use the guest console or `dbg shell` to run `dlv` against a target process or binary

Example (inside guest):

```bash
dlv exec /bin/<tool>
```

> Depending on how you want to attach, you may need additional port forwarding or to run Delve in headless mode and forward its port via QEMU. This is a common approach for Go userspace debugging once the workflow is finalized.

---

## Testing

Run unit tests on the host:

```bash
go test ./...
```

Verbose:

```bash
go test -v ./...
```

Because avyos includes OS-level behavior, many changes should also be validated by booting in QEMU and testing end-to-end behavior (service startup, IPC, UI, etc.).

---

## Profiling

There is no dedicated profiling target yet, but standard Go tooling works:

### Package-level CPU profile (tests)
```bash
go test ./lib/... -run TestName -cpuprofile cpu.out
go tool pprof cpu.out
```

### Benchmarks with profiles
```bash
go test ./... -bench . -benchmem -cpuprofile cpu.out -memprofile mem.out
go tool pprof cpu.out
go tool pprof mem.out
```

For runtime profiling of services/apps inside the guest, the preferred direction is to use:
- targeted instrumentation in the service/app, or
- a dedicated profiling service/tool exposed via Sutra,
as the project evolves.

---

## Troubleshooting

- If QEMU fails to boot, check:
  - `_cache/<arch>/qemu-debug.log` (when `DEBUG=1`)
  - your `external/<arch>/` firmware + kernel assets
- If `dbg` cannot connect:
  - confirm QEMU is running with the default forwarded port (`DBG_PORT`)
  - confirm `dbgd` is running in the guest and listening on `:5037`
