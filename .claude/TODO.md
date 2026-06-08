# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [x] **Go public** — repo is public as of 2026-05-25
- [ ] **Enable GitHub Pages** — Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## Features

- [x] **Unified action system** — implemented. `@action <type>`, `actions:` key in `.dagger`, aliases (`@source`/`@no-source`/`@symlink`/`compose: true`), sequencing validation all done. Convention dirs use explicit `.dagger` defaults by design — not implicit magic.
- [x] **`@disable` annotation** — implemented. Walk skips disabled files; disabled paths logged at debug level.
- [x] **BasicNode completeness** — `after`, `require`, `request`, `disable` all expressible in `.dagger` `files:` dict. `internal/manifest` and §20 dropped.
- [x] **`compose: true` alias** — works as shorthand for `composition.enabled: true` in `.dagger`.
- [ ] **TTY-aware missing-key prompt** (M3) — currently always halts with hint; no interactive fallback. Deferred.
- [x] **`dotd init` rc-file check** (M8) — `maybeAddSourceLine` wired into `runInit`. Reads shell/os from resolved env, uses `setup.DetectShellConfig`/`HasSourceLine`/`AppendSourceLine`.

## Code Quality

- [ ] cmd/dotd: `buildActOptions` returns `(ActOptions, error)` but error is now always nil after AUDIT-001; drop the error return and clean up the three call sites (`runPipeline`, `compose_cmd.go`, `unapply_cmd.go`).
- [ ] cmd/dotd: `dotcfg.DefaultPath()` now has only one caller (`teardown_cmd.go`, deliberate exception per AUDIT-003); consider replacing with direct `ecosystem.DefaultConfigFile()` call and deleting the wrapper.
- [ ] cmd/dotd test helper: `run()` at `main_test.go:14` could accept `io.Reader` for stdin so tests covering interactive commands (setup, init) don't need to reinvent the cobra wiring.

## UX / Help

- [ ] `dotd concepts`: add sub-topic routing (`dotd concepts when`, `dotd concepts env`, etc.) once the flat version is validated with users

## Git / CI Infrastructure

- [x] Multi-distro integration testing via Docker — Ubuntu e2e done (PRs #77–78, v0.2.34). Three tests: binary, installer, combined. Failure opens GH issue. Fedora deferred.
- [ ] **Go public** note: Done. `install.sh` now curl-only (PR #76).
