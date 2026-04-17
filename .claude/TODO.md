# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Project Setup

- [x] Add `LICENSE` (MIT)
- [x] Initialize `go.mod` (module `github.com/rocne/dot-dagger`)

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] Remove `-H "Authorization: Bearer $(gh auth token)"` from install one-liner in README and install.sh comments — only needed while repo is private. Do this when repo goes public.
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## UX / Polish

- [x] `dotd setup` — interactive onboarding command. Scaffolds repo structure, env.yaml, .dot-dagger.yaml, packages.yaml. Shell hook detection + auto-append. `--yes`/`--no-interactive` flags.
- [x] Colorization and theming — done: `internal/ui` package, semantic colors across all tools, colored cobra help output (bold headers, cyan commands/flags).

## Git / CI Infrastructure

- [x] GitHub Actions CI workflow (build + test)
- [x] GitHub Actions lint workflow (`golangci-lint`)
- [x] PR template (`.github/pull_request_template.md`)
- [x] Integration tests with real fixture dotfiles repo (`go test -tags integration ./cmd/dotd/`)
- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)

## Implementation

- [x] Phase 0 — repo bootstrap (LICENSE, go.mod, skeleton, CI, lint, PR template)
- [x] Phase 1 — `internal/annotation`, `internal/predicate`
- [x] Phase 2 — `internal/daggeryaml`, `internal/env`
- [x] Phase 3 — `internal/walk`, `internal/fileset`
- [x] Phase 4 — `internal/dag`, `internal/initgen`, `internal/linker`, `internal/packages`
- [x] Phase 5 — `cmd/dotd` (single binary: env, dag, link, package, setup, adopt)

---

## Architecture / Design

- [x] Single binary vs suite — resolved: single `dotd` binary with subcommands.
- [x] Config file naming — resolved: `.dot-dagger.yaml`, sections: `dotd`, `link`, `env`.
- [x] Unknown annotation/predicate behavior — resolved: warns or errors (configurable), never silently false.
- [x] Review `link_root` and `@symlink` relative path semantics — resolved: `link_root` cascades from `.dot-dagger.yaml` `link` section; inner overrides outer; empty = fallback to `Options.LinkRoot`. `@symlink` relative paths resolve against effective `link_root`.
- [x] Package install approach — resolved: `dotd package generate` emits a shell script; user runs `dotd package generate | sudo sh`. No subprocess exec from binary.
