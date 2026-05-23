# Developers Guide

This document is for contributors working on the avyos codebase day-to-day: how to navigate the repo, how to add services and APIs, common workflows, and debugging tips.

If you’re new, start with:
- [`README.md`](../README.md) — project overview
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — system architecture + code map
- [`BUILDING.md`](./BUILDING.md) — build/run/debug instructions

---

## 1. Repo orientation

### Key directories
- `bin/` and `sbin/` — system commands and entrypoints (including PID 1 init and management tools)
- `libexec/` — long-running daemons (system capabilities live here)
- `api/` — Sutra API schemas (`api.json`)
- `lib/` — shared Go packages (Sutra, graphics, filesystem helpers, etc.)
- `tools/` — generators and build/dev tools (`apigen`, image tools, `dbg`)
- `docs/` — documentation and assets
- `external/` — boot assets, kernel images, firmware, etc.

### Naming conventions
- Services: `libexec/<name>/...`
- API schema: `api/<name>/api.json` (same `<name>` as the service)
- Command/tool: `bin/<tool>/...` or `sbin/<tool>/...`
- Shared library: `lib/<area>/...`

---

## 2. Day-to-day workflows

### Build and run
See [`BUILDING.md`](./BUILDING.md).

### Fast iteration tips
- Use `make compile_db` if you’re changing build dependencies frequently.
- Prefer small PRs and keep changes localized to one subsystem where possible.
- When modifying a service API, regenerate bindings and update both client + server usage.

---

## 3. Sutra and API development

### 3.1 How Sutra is used
- Each service typically exposes a unix socket under `/cache/runtime`
- Clients connect, send requests, and receive events
- Type-safe bindings are generated from API schemas

### 3.2 Adding or changing a service API
1. Create/update schema:
   - `api/<service>/api.json`
2. Run the generator:
   - `go run ./tools/apigen` (or the project’s preferred wrapper/target)
3. Update:
   - server handlers in `libexec/<service>/`
   - client calls in commands/apps/libexec
4. Boot in QEMU and validate end-to-end.

> Rule of thumb: if more than one client uses it, it should be a schema + generated binding, not an ad-hoc message format.

### 3.3 API schema hygiene
- Keep requests/responses minimal and explicit
- Prefer additive changes (new fields, new messages) over breaking renames
- Document new API messages in code comments near handlers (or in `/docs` if intended for external use)

---

## 4. Writing a new service

### 4.1 Skeleton
A typical service consists of:
- a main entrypoint under `libexec/<name>/`
- configuration under `/avyos/etc/<name>/` (defaults) and `/etc/<name>/` (overrides)
- an API schema (if clients call it)

### 4.2 Service lifecycle integration
A service should:
- log clearly on startup and on fatal exit
- create its Sutra endpoint under `/cache/runtime`
- run under a dedicated service account where applicable
- be startable by PID 1 init / service manager

---

## 5. Applications

### 5.1 App bundle layout
Apps live under `/apps/<id>/`:
- `manifest.json`
- `exec` (native binary or AppImage)
- `icon.png` (optional)

### 5.2 Service boundary
Apps should not directly manipulate privileged resources. The intended model is:
- app → Sutra → service (authz enforced by service)

---

## 6. Linux distro compatibility layer (`/linux`)

The optional distro rootfs provides access to “normal Linux userland” resources (libraries, codecs, tools) inside a restricted container.

Developer notes:
- the layer is managed by `sbin/distro`
- keep the container boundary strict: avoid accidental coupling to host paths
- treat `/linux` as least-privileged

---

## 7. Graphics development

### Components
- `libexec/desktop` — compositor/display server
- `api/display` — display protocol schema
- `lib/graphics` — client-side framework (QML-like UI files, rendering, input, assets)

### Workflow
- Extend schema in `api/display/api.json`
- Regenerate bindings
- Implement server-side behavior in `libexec/desktop`
- Update client framework in `lib/graphics`

---

## 8. Debugging workflows

See the detailed guide in [`BUILDING.md`](./BUILDING.md). Highlights:

- QEMU `DEBUG=1` for low-level debugging (`-s -S`, gdbstub on :1234)
- `tools/dbg` client to interact with the guest via `libexec/dbgd`
- guest-side tools (e.g., `dlv`) for Go userspace debugging

---

## 9. Standards

### Formatting
- `gofmt` is required
- keep imports clean and deterministic

### Logging
- services: structured/consistent logging (avoid noisy prints)
- commands: user-friendly output for interactive usage, machine-friendly modes where needed

### Error handling
- return errors with context
- do not swallow errors in system-critical paths (boot/service start)

---

## 10. Where to put new documentation

- `ARCHITECTURE.md`: system decomposition and stable “code map”
- `README.md`: non-technical overview + navigation links
- `BUILDING.md`: build/run/debug instructions
- `DEVELOPERS.md`: contributor workflows (this document)
- subsystem docs: `docs/<topic>.md` when details would clutter these top-level docs
