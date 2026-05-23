<p align="center">
  <img src="data/icons/logo/logo.png" width="180" alt="avyos Logo"/>
</p>

<h1 align="center">avyos</h1>

<p align="center">
  A Linux-based operating system written entirely in pure Go —<br/>
  no external packages, no CGO, no POSIX compatibility.
</p>

<p align="center">
  <img src="docs/assets/interface.png" width="60%" alt="Interface"/>
</p>


## What is avyos?

**avyos** is an operating system project. It uses the **Linux kernel** for hardware support and booting, but rethinks the “system layer” (init, services, core tools, UI stack) in **pure Go** with **CGO disabled**.

If you’re not an OS person: you can think of avyos as “a Linux-based OS image with its own Go-written system core”.

### What makes it different?

- **Immutable system core**  
  The OS core is mounted at `/avyos` and treated as read-only at runtime.

- **Clear separation of state**  
  Apps, configuration, user data, and runtime files live in separate writable locations (`/apps`, `/config`, `/users`, `/cache`).

- **Service-driven capabilities**  
  Instead of letting apps poke low-level system resources directly, privileged operations are intended to be exposed via services and accessed through authenticated IPC.

- **Optional Linux compatibility layer**  
  A distro root filesystem can be provided at `/linux` and run in a restricted container for compatibility and reuse of existing Linux userland resources.

For the detailed system breakdown and code map, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

---

## Try avyos (no source code required)

You can try avyos in QEMU using **prebuilt release images**.

### Option A: One-command runner (recommended)

This tool downloads the right release ZIP for your architecture and starts QEMU:

```bash
go run avyos.dev/tools/runimage@latest
```

Supported flags (from `tools/runimage`):

- `--arch <arch>`: target architecture (default: your host `GOARCH`, e.g. `amd64`, `arm64`)
- `--branch <name>`: release tag/branch to download (default: `main`)
- `--cpu <n>`: CPU cores for QEMU (default: `2`)
- `--memory <size>`: RAM for QEMU (default: `2G`)
- `--vnc <display>`: start VNC server (example `:0`)
- `--dbg-port <port>`: forward `host:port → guest:5037` (default: `5037`, `0` disables)

Any extra arguments after the flags are passed directly to QEMU.

Examples:

```bash
# Run with VNC on :0
go run avyos.dev/tools/runimage@latest --vnc :0

# Use 4 cores and 4G RAM
go run avyos.dev/tools/runimage@latest --cpu 4 --memory 4G

# Disable dbgd port forwarding
go run avyos.dev/tools/runimage@latest --dbg-port 0
```

> Requirements: Go + QEMU installed on your machine.

### Option B: Download a release ZIP and run QEMU manually

Releases are published on GitHub [Releases page](https://github.com/itsManjeet/avyos/releases)

Each release provides assets **arch-specific ZIP** named like:

- `avyos-<release>-amd64.zip`
- `avyos-<release>-arm64.zip`

The ZIP contains:
- `disk.img`
- `firmware`
- `variables`

Unzip it, then run QEMU from that directory.

#### amd64

```bash
qemu-system-x86_64    \
  -smp 2 -m 2G        \
  -serial mon:stdio   \
  -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:5037-:5037   \
  -vga none           \
  -device virtio-gpu-pci \
  -device virtio-keyboard-pci \
  -device virtio-mouse-pci   \
  -drive if=pflash,file=firmware,readonly=on,format=raw \
  -drive if=pflash,file=variables,format=raw \
  -drive file=disk.img,format=raw
```

#### arm64

```bash
qemu-system-aarch64   \
  -M virt -cpu cortex-a57   \
  -smp 2 -m 2G        \
  -serial mon:stdio   \
  -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:5037-:5037   \
  -vga none           \
  -device virtio-gpu-pci \
  -device virtio-keyboard-pci \
  -device virtio-mouse-pci   \
  -drive if=pflash,file=firmware,readonly=on,format=raw \
  -drive if=pflash,file=variables,format=raw \
  -drive file=disk.img,format=raw
```

Notes:
- The networking rule forwards host TCP `5037` to the guest `dbgd` service on `5037`. Remove the `hostfwd=...` part if you don’t want it.
- Add `-vnc :0` to enable VNC output.

---

## Docs

- **Building and running from source**: [`BUILDING.md`](./docs/BUILDING.md)
- **Architecture**: [`ARCHITECTURE.md`](./docs/ARCHITECTURE.md)
- **Developer workflows**: [`DEVELOPERS.md`](./docs/DEVELOPERS.md)
- **Contributing**: [`CONTRIBUTING.md`](./docs/CONTRIBUTING.md)

---

## License

See [`LICENSE`](./LICENSE).
