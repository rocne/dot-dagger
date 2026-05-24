# Handoff — 2026-05-24 — XDG Canonical Path Audit

## State at handoff

**Branch:** `main` — all work merged via PR #69.

**Open PRs:** none.

---

## What was done this session

### PR #69 — `feature/claude-xdg-canonical-paths` (merged)

Two commits:

**Commit 1 — Bug 2 fix (macOS paths)**
`fix: route config/env DefaultPath through ecosystem XDG interface`

`os.UserConfigDir()` returns `~/Library/Application Support` on macOS, ignoring `XDG_CONFIG_HOME`. Both `internal/config/config.go:DefaultPath()` and `internal/env/env.go:DefaultPath()` called it directly.

Fix: added `ecosystem.DefaultConfigFile()` alongside the existing `ecosystem.DefaultEnvFile()`. Both `DefaultPath()` functions now delegate to ecosystem. One place to fix: `ecosystem.xdgConfigHome()`.

**Commit 2 — P0 canonical resolution audit**
`fix: p0 audit — eliminate all canonical resolution violations`

Four-team parallel agent audit across: raw OS path calls, env var reads, Default* call sites, runtime/OS detection. Found five violations, all fixed:

| # | File | Violation | Fix |
|---|------|-----------|-----|
| 1 | `cmd/dotd/config_cmd.go` | `dotcfg.DefaultPath()` re-called inside config subcommands after `resolvePaths` already ran | Added `configPath string` to `appConfig`; `resolvePaths` stores it; config subcommands read `cfg.configPath` |
| 2 | `cmd/dotd/setup_cmd.go` | `env.DefaultPath()` called when `cfg.envFile` already resolved | Replaced with `cfg.envFile` |
| 3 | `internal/pipeline/act.go` | `os.UserHomeDir()` fallback when `ActOptions.HomeDir` is empty | `HomeDir` is now required; returns error if empty. Adopter and pipeline tests updated to always set it. |
| 4 | `internal/setup/shell.go` | `os.UserHomeDir()` in `DetectShellConfig` and `SourceLine` | Both functions now accept `home string` as a parameter; callers pass `cfg.linkRoot` |
| 5 | `internal/setup/shell.go` | `os.Getenv("XDG_CONFIG_HOME")` for fish RC path | Replaced with `ecosystem.XdgConfigHome()` — a new exported canonical wrapper added to ecosystem |

**Clean areas (no violations):**
- `runtime.GOOS` — getters.go only ✅
- `os.Hostname()` — getters.go only ✅
- `DOTD_*` env vars — all in `ecosystem.ResolvePath` ✅
- `ecosystem.Default*` calls — all in `resolvePaths()` ✅
- `$EDITOR` — standard Unix convention, legitimate ✅

---

## Architecture state after this session

The canonical resolution rules are now fully enforced across the codebase:

**Path chain:** CLI flag → `DOTD_*` env var → env.yaml field → `ecosystem.DefaultX()` → resolved once in `resolvePaths()` → stored in `cfg.*` → command code reads `cfg.*` only.

**Env values chain:** env.yaml → `resolveEnv(cfg)` → resolved map → commands read from map.

**OS/shell/hostname:** `getters.go` only, exposed via `dotd get-os` / `dotd get-hostname` shell expressions in env.yaml.

**XDG paths:** All route through `ecosystem.xdgConfigHome()` / `ecosystem.xdgDataHome()`. Exported as `ecosystem.XdgConfigHome()` for use by packages that need the raw base (currently `internal/setup/shell.go` for fish RC path).

---

## Known deferred items

### Bug 1 — teardown pre-checks scan cwd when dotfiles not configured

In `teardown_cmd.go`, `hasDaggerFiles(cfg.files)` and `runPipeline(cfg, true)` both run as pre-flight checks even when no dotfiles repo is configured (i.e., `cfg.files` defaults to cwd). On macOS this triggers permission prompts for every folder under cwd.

**Fix:** skip pre-checks when `existing.Dotfiles` is empty:
```go
// in runTeardown, before pre-checks:
configPath, _ := dotcfg.DefaultPath()   // bootstrap exception — deliberate
existing, _ := dotcfg.Load(configPath)
if existing.Dotfiles == "" {
    // skip hasDaggerFiles and runPipeline pre-checks
}
```

This is a deliberate exception (bootstrap context — reading config.yaml before tearing it down).

### Logging

New commands (`setup`, `teardown`, `unapply`) were written without structured logging. They use `fmt.Fprintf(cmd.OutOrStdout(), ...)` for all output. The goal is to use `cfg.log.Debugf`, `cfg.log.Infof`, `cfg.log.Warnf` consistently, matching the existing pattern in `runApply` / `runCheck`.

Not urgent but the user wants it done. No design decisions needed — just wire `cfg.log` into the new commands the same way `runApply` uses it.

---

## Files modified this session

| File | Change |
|------|--------|
| `internal/ecosystem/ecosystem.go` | Added `DefaultConfigFile()`, exported `XdgConfigHome()` |
| `internal/config/config.go` | `DefaultPath()` now delegates to `ecosystem.DefaultConfigFile()` |
| `internal/env/env.go` | `DefaultPath()` now delegates to `ecosystem.DefaultEnvFile()` |
| `internal/setup/shell.go` | `DetectShellConfig`/`SourceLine`/`AppendSourceLine` now accept `home string`; fish uses `ecosystem.XdgConfigHome()` |
| `internal/pipeline/act.go` | `HomeDir` required; `os.UserHomeDir()` fallback removed |
| `internal/pipeline/act_test.go` | Tests now set `HomeDir` |
| `internal/adopter/adopter_test.go` | Tests now set `LinkRoot` where missing |
| `cmd/dotd/main.go` | Added `configPath string` to `appConfig`; stored in `resolvePaths`; `newConfigCmd(cfg)` |
| `cmd/dotd/config_cmd.go` | All subcommands accept `cfg *config`; use `cfg.configPath` |
| `cmd/dotd/setup_cmd.go` | `env.DefaultPath()` → `cfg.envFile`; removed `env` import |
| `cmd/dotd/teardown_cmd.go` | `DetectShellConfig` call passes `cfg.linkRoot` |
