# Code Audit

Findings from full codebase review. Each item has a status and priority.

Statuses: `open` | `in-progress` | `done` | `wont-fix` | `deferred`

---

## H — Spec Violations

### A-01 — Config file named `.dotr.yaml` everywhere; should be ecosystem-named
**Status:** `done`  
**Files:** `internal/daggeryaml/daggeryaml.go` (was `dotryaml`), `internal/walk/walk.go`, `internal/setup/setup.go`  
**Detail:** File is ecosystem config (sections owned by multiple tools), so it belongs to the ecosystem, not any one tool. Spec called it `.dotd.yaml` (also wrong — implies dotd ownership). Renamed to `.dot-dagger.yaml` using `ecosystem.ConfigFile`. Package renamed `dotryaml` → `daggeryaml`, struct `DotR` → `Config`.

### A-02 — `dotr` init.sh default path diverges from `dotd`
**Status:** `done` — noted as part of A-09 ecosystem canonicalization  
**Files:** `cmd/dotr/main.go:374` vs `cmd/dotd/main.go:344`  
**Detail:** `dotd` defaults to `~/.local/share/dot-dagger/init.sh` (correct per spec). `dotr` defaults to `~/.local/share/dot-dagger/init.sh` (wrong). Both write the same file; divergent defaults break interoperability without `--init-file`.  
**Fix:** Change `dotr defaultInitFile()` to return `~/.local/share/dot-dagger/init.sh`. Resolved by A-09 (single source of truth for default paths).

---

## H — Silent Failures

### A-03 — `os.UserHomeDir()` errors silently discarded in 4+ places
**Status:** `done`  
**Detail:** `expandTilde` in walk, `DetectShellConfig`/`SourceLine` in setup/shell, `expandHome` in cmd/dotr/setup — all now propagate errors. Default path functions already centralised by A-09.

---

## M — Bad Duplication

### A-04 — `resolveEnv()` duplicated between `cmd/dotd` and `cmd/dotr`
**Status:** `done`  
**Detail:** `env.ResolveWithOverrides(envFilePath, kvOverrides)` exported. Both cmds now one-liners.

### A-05 — Default path functions duplicated across all cmd packages
**Status:** `done`  
**Files:** `cmd/dot{d,e,l,p,r}/main.go` — each defines its own `defaultDotfiles()`, `defaultEnvFile()`, `defaultInitFile()`  
**Detail:** Identical or nearly identical functions in every cmd package. No single source of truth.  
**Fix:** Centralise in `internal/ecosystem` (see A-09). Each cmd imports and calls them.

### A-06 — Symlink state reporting duplicated: `cmd/dotl` vs `cmd/dotr/link.go`
**Status:** `done`  
**Detail:** `linker.PrintCheckSummary`, `linker.PrintRemovePlan`, `linker.CountOwned` extracted. Both cmds now delegate to these.

### A-07 — Package install loop duplicated: `cmd/dotp` vs `cmd/dotr/main.go`
**Status:** `done`  
**Detail:** `packages.InstallOne` and `packages.ResolveInstallCmd` extracted. Both cmds now delegate; tool name passed for error messages.

---

## M — Magic Values / Ownership

### A-08 — Directory names `"scripts"`, `"conf"`, `"bin"` are raw strings with no owner
**Status:** `done`  
**Detail:** `walk.DirScripts`, `walk.DirConf`, `walk.DirBin` exported. Used in walk internals and setup scaffold.

### A-09 — Ecosystem name and default config paths have no single owner
**Status:** `done`  
**Files:** `cmd/dotd/main.go:339,344`, `cmd/dotr/main.go:369,374`, `cmd/dote/main.go:93` — all hardcode `"dot-dagger"` and construct paths independently  
**Detail:** String `"dot-dagger"` duplicated as raw literal. No constant. Default path logic (env.yaml, init.sh) scattered. `DOTFILES` env var lookup also duplicated.  
**Fix:** Create `internal/ecosystem/ecosystem.go` with:
- `const Name = "dot-dagger"`
- `func DefaultEnvFile() (string, error)`
- `func DefaultInitFile() (string, error)`
- `func DefaultDotfiles() string` (reads `$DOTFILES`, falls back to cwd)

All cmd packages import and use these. **This is the current work item.**

---

## L — Incomplete / Stale

### A-10 — `dotd install` is a stub
**Status:** `done`  
**Detail:** Removed. Setup lives in `dotr setup`.

### A-11 — `internal/annotation/registry.go` is unused dead code
**Status:** `done`  
**Detail:** Deleted. Tests removed from annotation_test.go.

### A-12 — `config.yaml` spec feature unimplemented
**Status:** `deferred`  
**Files:** Spec: `internal/env/env.md` (§7) — directory name overrides, `bin_dir`, `dirs`  
**Detail:** Spec defines `config.yaml` for power-user customisation. No code reads it. Users cannot override dir names.  
**Fix:** Deferred. Track as known gap. Implement when dir name customisation is needed.
