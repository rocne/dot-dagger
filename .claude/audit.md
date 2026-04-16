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
**Status:** `open`  
**Files:** `cmd/dotr/main.go:368,373`, `cmd/dotd/main.go:339,344`, `cmd/dote/main.go:93`, `internal/walk/walk.go` (expandTilde), `internal/setup/shell.go:21,41`  
**Detail:** Pattern `home, _ := os.UserHomeDir()` everywhere. If home dir resolution fails, paths silently become `/init.sh`, `/.config/...` etc — silent corruption.  
**Fix:** Propagate the error. Default path functions should return `(string, error)`. Resolved by A-09 (centralized path functions that return errors).

---

## M — Bad Duplication

### A-04 — `resolveEnv()` duplicated between `cmd/dotd` and `cmd/dotr`
**Status:** `open`  
**Files:** `cmd/dotd/main.go:292`, `cmd/dotr/main.go:256`  
**Detail:** Identical 18-line functions. Constructs overrides from env.yaml + `--env` flags, calls `env.NewResolver().Resolve()`.  
**Fix:** Export a helper in `internal/env` — e.g. `ResolveWithOverrides(envFilePath string, kvOverrides []string) (map[string]string, error)`.

### A-05 — Default path functions duplicated across all cmd packages
**Status:** `done`  
**Files:** `cmd/dot{d,e,l,p,r}/main.go` — each defines its own `defaultDotfiles()`, `defaultEnvFile()`, `defaultInitFile()`  
**Detail:** Identical or nearly identical functions in every cmd package. No single source of truth.  
**Fix:** Centralise in `internal/ecosystem` (see A-09). Each cmd imports and calls them.

### A-06 — Symlink state reporting duplicated: `cmd/dotl` vs `cmd/dotr/link.go`
**Status:** `open`  
**Files:** `cmd/dotl/main.go:runApply,runCheck,runRemove`, `cmd/dotr/link.go:runLinkApply,runLinkCheck,runLinkRemove`  
**Detail:** Near-identical logic. Only difference: `dotl` uses unfiltered fileset; `dotr link` uses predicate-filtered. Output formatting, error handling, flag handling all duplicated.  
**Fix:** Extract `linker.PrintLinkStates(cmd, links)` and similar helpers into `internal/linker` or `internal/ui`. The run* functions call shared helpers and only differ in how they build the fileset.

### A-07 — Package install loop duplicated: `cmd/dotp` vs `cmd/dotr/main.go`
**Status:** `open`  
**Files:** `cmd/dotp/main.go:runInstall`, `cmd/dotr/main.go:handlePackage`  
**Detail:** Same install/check/skip logic duplicated. Error messages differ slightly (`dotp:` vs `dotr:`).  
**Fix:** Export `packages.Install(cmd, req, reg, dryRun, verbose)` in `internal/packages`. Both callers delegate to it.

---

## M — Magic Values / Ownership

### A-08 — Directory names `"scripts"`, `"conf"`, `"bin"` are raw strings with no owner
**Status:** `open`  
**Files:** `internal/walk/walk.go:73-77` (primary), referenced in `internal/setup/setup.go` scaffold, comments throughout  
**Detail:** No named constants. Any future rename requires grep-and-replace across packages. Walk package is the right owner.  
**Fix:** Export constants from `internal/walk`: `DirScripts`, `DirConf`, `DirBin`. Use them everywhere.

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
**Status:** `open`  
**Files:** `cmd/dotd/main.go:79-86`  
**Detail:** Prints "not yet implemented", returns nil. Either implement (wire to `dotr setup` logic) or remove the command.  
**Fix:** Remove command until it's ready, or redirect: `"use dotr setup instead"`.

### A-11 — `internal/annotation/registry.go` is unused dead code
**Status:** `open`  
**Files:** `internal/annotation/registry.go`  
**Detail:** Registry type defined and exported, never instantiated in any cmd or internal package.  
**Fix:** Remove unless there's a concrete near-term plan for custom annotation handlers.

### A-12 — `config.yaml` spec feature unimplemented
**Status:** `deferred`  
**Files:** Spec: `internal/env/env.md` (§7) — directory name overrides, `bin_dir`, `dirs`  
**Detail:** Spec defines `config.yaml` for power-user customisation. No code reads it. Users cannot override dir names.  
**Fix:** Deferred. Track as known gap. Implement when dir name customisation is needed.
