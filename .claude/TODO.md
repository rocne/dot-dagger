# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Project Setup

- [x] Add `LICENSE` (MIT)
- [x] Initialize `go.mod` (module `github.com/rocne/dot-dagger` — rename deferred)

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] Remove `-H "Authorization: Bearer $(gh auth token)"` from install one-liner in README and install.sh comments — only needed while repo is private. Do this when repo goes public.
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## UX / Polish

- [x] `dotr setup` — interactive onboarding command. Scaffolds repo structure, env.yaml, .dot-dagger.yaml, packages.yaml. Shell hook detection + auto-append. `--yes`/`--no-interactive` flags.
- [x] Colorization and theming — done: `internal/ui` package, semantic colors across all tools, colored cobra help output (bold headers, cyan commands/flags).

## Git / CI Infrastructure

- [x] GitHub Actions CI workflow (build + test)
- [x] GitHub Actions lint workflow (`golangci-lint`)
- [x] PR template (`.github/pull_request_template.md`)

## Implementation

See `.claude/docs/implementation-plan.md` for the full phased plan.

- [x] Phase 0 — repo bootstrap (LICENSE, go.mod, skeleton, CI, lint, PR template)
- [x] Phase 1 — `internal/annotation`, `internal/predicate`
- [x] Phase 2 — `internal/daggeryaml`, `internal/env`, `cmd/dote`
- [x] Phase 3 — `internal/walk`, `internal/fileset`
- [x] Phase 4 — `internal/dag`, `internal/initgen`, `internal/linker`, `internal/packages`
- [x] Phase 5 — `cmd/dotd`, `cmd/dotl`, `cmd/dotp`, `cmd/dotr`

---

## Architecture / Design

- [x] Consider splitting dot-dagger into separate tools — resolved: suite of 6 tools (`dota`, `dote`, `dotd`, `dotl`, `dotp`, `dotr`). See `.claude/docs/specv2/`.
- [x] Decide `env.yaml` ownership — resolved: `dote` owns it. New dedicated library/tool for environment resolution.
- [x] Config file naming — resolved: `.dot-dagger.yaml`, sections namespaced by tool.
- [x] Standalone file selection for `dotl`/`dotp` — resolved: standalone = unconditional walk of owned dirs; orchestrated = receives filtered list from `dotd` via `dotr`.
- [x] Unknown annotation/predicate behavior — resolved: `dota` warns or errors (configurable), never silently false.
- [x] Review `link_root` and `@symlink` relative path semantics — resolved: `link_root` cascades from `.dot-dagger.yaml` `dotl` section; inner overrides outer; empty = fallback to `Options.LinkRoot`. `@symlink` relative paths resolve against effective `link_root`.
- [x] Spec out `dotp` fully — see `.claude/docs/specv2/dotp.md`.
- [x] Repo rename/retire — handled naturally as suite repos are created; not worth tracking.
