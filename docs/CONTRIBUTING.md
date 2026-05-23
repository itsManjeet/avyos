# Contributing to avyos

Thanks for your interest in contributing. This document explains how to propose changes, how the repo is organized, and the expectations for patches.

> If you’re new to the codebase, start with [`README.md`](./..README.md) and [`ARCHITECTURE.md`](./ARCHITECTURE.md).

---

## Ways to contribute

- Report bugs (boot issues, crashes, regressions, incorrect behavior)
- Propose features (filesystem layout, services, IPC APIs, graphics stack)
- Improve documentation
- Implement issues / roadmap items

---

## Project principles (what we try to preserve)

- **Immutable OS core:** `/avyos` is treated as read-only at runtime.
- **Service-driven capabilities:** privileged actions should be provided by services and accessed via Sutra.
- **Go-first userland:** CGO remains disabled; prefer pure-Go implementations.
- **Explicit APIs:** if something is consumed by multiple clients, define an API schema under `api/<name>/api.json` and generate bindings.

---

## Quick repository map

- `cmd/` — system commands and entrypoints (init, managers, tooling)
- `services/` — long-running system services (including `display`)
- `api/` — Sutra API schemas (`api.json`)
- `pkg/` — shared Go packages (IPC, graphics, helpers)
- `tools/` — codegen and build/dev tools
- `docs/` — documentation and assets
- `external/` — external artifacts (boot assets, kernel images)

---

## Before you open a PR

1. **Search existing issues/PRs** to avoid duplicates.
2. If the change affects system design (FS layout, IPC, service model), open a short design discussion first.
3. Keep PRs focused: one feature/fix per PR when possible.

---

## Coding guidelines

### Go
- `gofmt` everything.
- Prefer small packages with clear responsibilities.
- Avoid global state unless it’s a deliberate subsystem boundary.
- Keep error messages actionable; include context.

### CGO
- **No CGO.** If you think CGO is necessary, propose an alternative or open a design discussion.

### IPC / Sutra
When adding a new service API:
1. Add schema: `api/<service>/api.json`
2. Run/generate bindings via `tools/apigen`
3. Implement server in `services/<service>/`
4. Add client usage via generated bindings (avoid hand-rolled message formats)
5. Update docs when the API is meant for external consumption

### Filesystem + paths
- Do not hardcode absolute paths across the codebase.
- Prefer the project’s path resolution helpers when available.

---

## Testing and verification

At minimum:
- `go test ./...`

For runtime validation:
- boot in QEMU and validate the feature end-to-end (service starts, IPC works, UI/CLI behaves).

(See `BUILDING.md` for build/run instructions.)

---

## Commit messages

Use clear, descriptive commit messages. Good examples:
- `display: fix cursor hot-spot handling`
- `sutra: validate request payload length`
- `init: mount cache/runtime before starting services`

---

## Security-sensitive changes

If your change touches:
- authentication/authorization
- sandboxing (namespaces/caps)
- service privilege boundaries
- parsing of untrusted input (IPC, files, network)

…call it out in the PR description so it gets extra review.

---

## License

By contributing, you agree that your contributions will be licensed under the project’s license (see [`LICENSE`](./LICENSE)).
