# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Project Setup

- [ ] Add `LICENSE` (MIT decided, just needs the file)
- [ ] Initialize `go.mod` (defer until first `.go` file is written)

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors

## Git / CI Infrastructure

- [ ] GitHub Actions CI workflow (build + test) — defer until `go.mod` exists
- [ ] GitHub Actions lint workflow (`golangci-lint`) — defer until `go.mod` exists
- [ ] PR template (`.github/pull_request_template.md`)

## Implementation

See `.claude/docs/implementation-plan.md` for the full phased plan.

- [ ] Phase 0 — repo bootstrap (LICENSE, go.mod, skeleton, CI, lint, PR template)
- [ ] Phase 1 — `internal/annotation`, `internal/predicate`
- [ ] Phase 2 — `internal/dotryaml`, `internal/env`, `cmd/dote`
- [ ] Phase 3 — `internal/walk`, `internal/fileset`
- [ ] Phase 4 — `internal/dag`, `internal/initgen`, `internal/linker`, `internal/packages`
- [ ] Phase 5 — `cmd/dotd`, `cmd/dotl`, `cmd/dotp`, `cmd/dotr`

---

## Architecture / Design

- [x] Consider splitting dot-dagger into separate tools — resolved: suite of 6 tools (`dota`, `dote`, `dotd`, `dotl`, `dotp`, `dotr`). See `.claude/docs/specv2/`.
- [x] Decide `env.yaml` ownership — resolved: `dote` owns it. New dedicated library/tool for environment resolution.
- [x] Config file naming — resolved: `.dotr.yaml`, sections namespaced by tool.
- [x] Standalone file selection for `dotl`/`dotp` — resolved: standalone = unconditional walk of owned dirs; orchestrated = receives filtered list from `dotd` via `dotr`.
- [x] Unknown annotation/predicate behavior — resolved: `dota` warns or errors (configurable), never silently false.
- [ ] Review `link_root` and `@symlink` relative path semantics more carefully before finalising spec. Current resolution: `@symlink` destinations are implicit-relative to `link_root` if they don't start with `/` or `~/`. Needs validation against real use cases.
- [x] Spec out `dotp` fully — see `.claude/docs/specv2/dotp.md`.
- [x] Repo rename/retire — handled naturally as suite repos are created; not worth tracking.
