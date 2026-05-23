# Session Handoff — 2026-05-23

## What's in flight

**PR #65** (`feature/claude-init-rc-canonical` → `main`) — open, not yet merged.

---

## What changed

### M8 — `dotd init` RC file wiring

`maybeAddSourceLine` added to `init_cmd.go`. After writing config/env files, `dotd init` now:
1. Calls `resolveEnv(cfg)` to get `shell` and `os` from the resolved env map
2. Uses `cfg.initFile` (already resolved via standard chain) for the init.sh path
3. Calls `setup.DetectShellConfig` / `HasSourceLine` / `AppendSourceLine`
4. Interactive: prompts Y/n. `--yes`: appends without prompt. Unrecognized shell: prints manual-add note.

`newInitCmd` now receives and passes `cfg` instead of discarding it (`_ *config`). Local `dotcfg.Config` var renamed `toolCfg` to avoid shadow.

### Canonical resolution audit + fixes

Two violations fixed:
- `envYamlPath` in `env.go` — was re-reading `DOTD_ENV_FILE` after `resolvePaths` had already resolved it. Now just `return cfg.envFile`.
- `buildActOptions` in `main.go` — had unreachable `os.UserHomeDir()` fallback (resolvePaths always sets `cfg.linkRoot`). Removed; function signature simplified to not return error.

Both callers of `buildActOptions` updated (`main.go`, `compose_cmd.go`).

### CLAUDE.md

Canonical resolution path philosophy documented in both:
- `~/CLAUDE.md` (global, project-agnostic)
- `dot-dagger/CLAUDE.md` (project-specific: the two chains, violations to avoid, legitimate exceptions)

### Repo went public

`rocne/dot-dagger` is now public. `brew tap rocne/tap && brew install dot-dagger` works.

---

## State of main

PR #65 is the only open work. Once merged, all known deferred items will be:
- M3 — TTY-aware missing-key prompt (still deferred)
- Docker integration testing (unblocked now that repo is public)

TODO.md is current.
