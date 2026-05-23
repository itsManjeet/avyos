# Architecture

This document is a **high-level description** of avyos: its major subsystems, how they interact, and where the corresponding code lives. It is intended to be stable over time and act as a “map” for contributors.

## 1. System overview

avyos is a Linux-kernel-based OS that boots via the Limine bootloader. The OS userland (init system, services, utilities, UI stack) is implemented primarily in Go with CGO disabled.

At runtime, the system is organized around:
- an immutable **OS core** mounted at `/avyos`
- writable **state + apps** mounted at `/etc`, `/apps`, `/users`, `/cache`
- a decentralized IPC mechanism (**Sutra**) used for service↔client communication
- a graphics stack implemented as a service (`display`) and a client framework (`lib/graphics`)
- an optional Linux distribution **compatibility layer** mounted at `/linux`

## 2. Privilege layers (security model at a glance)

avyos is designed as a layered system where *higher layers have less privilege* and must request privileged actions through services.

```
Least privilege
  ┌─────────────────────────────────────────────┐
  │ /linux compatibility layer (distro container)│
  │  - isolated rootfs (e.g., Debian)            │
  │  - no access to host environment by default  │
  │  - managed by sbin/distro                     │
  └─────────────────────────────────────────────┘
  ┌─────────────────────────────────────────────┐
  │ Applications layer (/apps)                   │
  │  - sandboxed with namespaces/caps            │
  │  - no direct system access                   │
  │  - interacts via authenticated services      │
  └─────────────────────────────────────────────┘
  ┌─────────────────────────────────────────────┐
  │ Commands layer (/usr/bin)                  │
  │  - user-privileged tools                     │
  │  - access to resources as the invoking user  │
  │  - primary operational interface             │
  └─────────────────────────────────────────────┘
  ┌─────────────────────────────────────────────┐
  │ Services / runtime layer (/avyos/libexec)   │
  │  - started/supervised by PID 1 init          │
  │  - run under dedicated service accounts      │
  │  - hold capabilities and broker access       │
  └─────────────────────────────────────────────┘
  ┌─────────────────────────────────────────────┐
  │ Kernel layer (Linux)                         │
  │  - highest privilege                         │
  │  - hardware drivers, scheduler, MM, etc.     │
  └─────────────────────────────────────────────┘
Most privilege
```

**Key rule:** Apps and the `/linux` distro container do not access system resources directly; they use **Sutra** to call privileged services that enforce authentication/authorization.

## 3. Boot and early userspace

### 3.1 Boot chain
1. Firmware / platform boot
2. Limine loads:
   - Linux kernel
   - initramfs (contains early init)
   - OS artifacts as defined by the build (image/rootfs)
3. Early init mounts required filesystems and transitions into the real root
4. The system `init` from the OS core starts as PID 1 (service supervision begins)

### 3.2 Early init responsibilities
Early init is responsible for:
- mounting kernel pseudo-filesystems and runtime tmpfs into avyos’ runtime layout
- mounting OS core and mutable partitions
- preparing runtime directories (e.g., sockets)
- executing the OS core init (`/avyos/sbin/init`)

## 4. Filesystem model

### 4.1 Immutable OS core
Mounted at `/avyos` and treated as read-only at runtime:

- `/usr/bin/`        Core utilities and system entrypoints
- `/avyos/libexec/`   Long-running daemons
- `/avyos/apps/`       OS-shipped apps (optional)
- `/avyos/etc/`     Factory defaults / fallback configuration
- `/avyos/share/`       OS-shipped shared data (usr-share-like)

### 4.2 Mutable partitions / writable state
- `/apps`    Installed applications
- `/etc`  System configuration overrides
- `/users`   User data / home directories
- `/cache`   Runtime state and caches (includes runtime sockets)
- `/linux`   Linux distribution rootfs compatibility layer (optional)

### 4.3 Kernel/runtime namespaces (avyos layout)
avyos places kernel pseudo-fs and runtime equivalents under `/cache`:

- `/proc`  Process information (proc-equivalent)
- `/sys`      sysfs-equivalent
- `/dev`    device nodes (devtmpfs-equivalent)
- `/cache/runtime`           runtime tmpfs (run-equivalent; sockets live here)

These locations are used internally by system components and services.

## 5. Services and IPC

### 5.1 Service model
System functionality is implemented as a set of independently executable services under:
- `/avyos/libexec/<name>/...`

Services are started and supervised by the init/service-management layer in `/usr/bin/`.
They run under **dedicated service accounts** and are the primary holders of privileged capabilities.

### 5.2 Sutra IPC
Sutra is the primary IPC mechanism used between services and clients.

Characteristics:
- decentralized: each service exposes its own endpoint (typically a unix socket)
- request/response method calls from client → service
- async events from service → client
- APIs defined via schema files under `/api/<name>/api.json`
- typed client/server bindings generated via `tools/apigen`

### 5.3 Service APIs
For each IPC-exposed service:
- schema: `/api/<name>/api.json`
- server: `/libexec/<name>/`
- generated bindings are used by:
  - server implementation (request decoding + event encoding)
  - clients (typed request builders + event handlers)

## 6. Core utilities (cmd)

The OS core ships CLI tools in `/usr/bin/` which implement system-facing functionality such as:
- init (PID 1)
- service management
- system inspection and admin tooling
- distro management (compatibility layer)

These utilities typically run with the privileges of the invoking user and access system functionality through services where required.

## 7. Linux distro compatibility layer (`/linux`)

avyos can provide a Linux distribution rootfs (e.g., Debian) mounted at `/linux` for:
- reusing existing libraries, codecs, and userland resources
- compatibility for third-party applications

This layer is managed by:
- `sbin/distro`

Execution model:
- processes run inside a **distro container**
- the container has **least privilege**
- no access to the host environment by default (no direct access to host services, devices, or files outside the compatibility boundary)

## 8. Application model (`/apps/<id>`)

Installed applications live under `/apps` as per-app directories:

`/apps/<id>/`
- `manifest.json`  Metadata and launch information
- `exec`           Self-contained executable payload (native binary or AppImage)
- `icon.png`       App icon (optional)

Execution model:
- apps are sandboxed (namespaces, capabilities, etc.)
- apps do not access system resources directly
- privileged actions are performed by calling authenticated services via Sutra

## 9. Graphics and display

### 9.1 Display service
The `display` service implements the system compositor/display server and exposes its API via:
- `/api/display/`
- `/libexec/display/`

It is responsible for:
- display discovery and modesetting
- buffer presentation
- input integration (where applicable)
- serving the custom display protocol

### 9.2 Graphics framework (`lib/graphics`)
`lib/graphics` provides the client-side graphics framework:
- declarative “QML-like” UI files describing elements + properties
- rendering pipeline targeting supported backends
- integration layer that speaks to the `display` service via generated API bindings

### 9.3 Backends
The display stack supports multiple backends (selected by platform/capability), including:
- KMS/DRM
- fbdev
- Wayland (where used)
- avyos custom display protocol

## 10. Repository map (code map)

Top-level directories (typical):

- `bin/` and `sbin/`
  System binaries and entrypoints (init, managers, admin tools, distro tools)

- `libexec/`
  Long-running daemons (display, system services)

- `api/`
  IPC API schemas (`api.json`) consumed by code generation

- `lib/`
  Shared Go packages (Sutra IPC, graphics framework, filesystem/layout helpers, etc.)

- `tools/`
  Developer/build tooling (e.g., `apigen`, image builders)

- `external/`
  External inputs/artifacts (kernel images, boot assets, etc.)

## 11. Key runtime flows

### 11.1 Boot → services
- early init prepares mounts and runtime directories
- PID 1 init starts essential services
- services create IPC endpoints under `/cache/runtime`
- clients connect and call methods via Sutra

### 11.2 Apps → services
- app starts (from `/apps/<id>/exec`)
- app calls system services via Sutra
- services enforce policy and broker access to privileged resources

### 11.3 Distro container → services
- distro environment is created/managed by `sbin/distro`
- processes inside the container use the same “service boundary” rule:
  - no direct host access
  - access via authenticated services when allowed
