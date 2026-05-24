# Handoff — 2026-05-23 — Lifecycle Commands

## State at handoff

**Current branch:** `feature/claude-init-walk-fixes` (but new work goes on a new branch — see below)

**Open PRs:**
- PR #66 — `feature/claude-cli-command-groups` — command groups in `dotd --help` (Core / Configuration / Advanced)
- PR #67 — `feature/claude-init-walk-fixes` — skip `.git` in Walk, normalize relative paths in init, improve wizard prompts

Both PRs are ready to merge. Neither is a dependency for the lifecycle commands work below.

---

## What was done this session

1. **Requirements doc written and locked** at `docs/superpowers/specs/2026-05-23-lifecycle-commands-requirements.md`
2. **Implementation plan written, challenged, and fixed** at `docs/superpowers/plans/2026-05-23-lifecycle-commands.md`

The plan is ready to execute. No code has been written yet for the lifecycle commands.

---

## The design (two-tier lifecycle)

| Tier | Set up | Tear down |
|------|--------|-----------|
| System (tool config) | `dotd setup` | `dotd teardown` |
| Reconcile (symlinks, init.sh) | `dotd apply` | `dotd unapply` |
| Directory scaffold | `dotd init` | _(manual — user's repo)_ |

Breaking changes to `dotd init` are acceptable and intentional.

---

## Plan summary (6 tasks, ready to execute)

**Start on a new branch:** `feature/claude-lifecycle-commands`

```bash
git checkout main
git pull
git checkout -b feature/claude-lifecycle-commands
```

### Task 1 — `RemoveSourceLine` in `internal/setup/shell.go`
New function that strips the dotd header + source line pair that `AppendSourceLine` writes. Tests in new `internal/setup/shell_test.go`.

### Task 2 — Rewrite `cmd/dotd/init_cmd.go`
Strip to scaffold-only: removes config/env/RC writing and `--yes` flag. Adds hard precondition check — if config.yaml absent, exits with `"no config found — run 'dotd setup' first"`. Keeps `scaffoldDaggerInteractive`, `scaffoldDagger`, `promptDefault`, `expandTildeStr`.

### Task 3 — Create `cmd/dotd/setup_cmd.go`
New interactive wizard. When config.yaml exists, loads current values and shows as per-field defaults. Uses `cfg.files`, `cfg.binDir`, `cfg.generatedDir`, `cfg.linkRoot` (resolved by `resolvePaths`) as fallback defaults — no direct env var lookups. Writes config.yaml and (if absent) env.yaml. Does NOT touch RC file.

### Task 4 — Create `cmd/dotd/teardown_cmd.go`
Removes config.yaml, env.yaml, RC source line. Pre-check warns on active symlinks (non-fatal if walk fails) and .dagger files. Shows preview + `[y/N]` prompt + `--yes` flag. Uses `dotcfg.DefaultPath()` / `env.DefaultPath()` directly — deliberate exception, removes system paths regardless of `--env-file` override. Fail-fast atomicity: config.yaml removed first; error before touching env.yaml or RC. Prunes config dir if empty.

### Task 5 — Create `cmd/dotd/unapply_cmd.go`
Reverses `apply`. Default mode: `runPipeline(cfg, true)` → check each `lnk.Dest` for symlink pointing to `lnk.Src` → queue removal. `--all` mode: walk all nodes (skip predicate filter), remove any symlink whose target is under `cfg.files`. Also removes init.sh if present. Preview + `[y/N]` + `--yes` + `--dry-run` support.

**Important:** Root command has a persistent `--all` flag (for `--help --all` to show hidden commands). Unapply's local `--all` is a different flag. pflag's `AddFlagSet` skips existing flags, so local wins during parsing. Root's `SetHelpFunc` reads `cmd.Root().PersistentFlags()` directly — unaffected. This is safe but add a comment in the code noting the shadow.

### Task 6 — Wire into `cmd/dotd/main.go`
Register `newSetupCmd(cfg)`, `newTeardownCmd(cfg)`, `newUnapplyCmd(cfg)`. Check `go build` and `dotd --help` look right.

---

## Key implementation details

### `AppendSourceLine` header string (exact match required)
```go
// In shell.go AppendSourceLine:
fmt.Fprintf(f, "\n# dotd — generated shell init\n%s\n", line)
// The — is an em dash (UTF-8: \xe2\x80\x94)

// In RemoveSourceLine, the const must match exactly:
const header = "# dotd — generated shell init"
```

### Canonical resolution in setup
`runSetup` must NOT call `os.LookupEnv("DOTFILES")` or hardcode default paths. Use the already-resolved `cfg.*` values:
```go
dotfilesDefault := existing.Dotfiles
if dotfilesDefault == "" {
    dotfilesDefault = cfg.files  // resolvePaths already handled DOTD_FILES → DOTFILES → default
}
// same pattern for cfg.binDir, cfg.generatedDir, cfg.linkRoot
```

### Teardown atomicity (no `canRemove`)
No pre-validation of delete permissions. Just stat to know what exists, show preview, confirm, then `os.Remove` in order (config.yaml → env.yaml → RC line). Any `os.Remove` error returns immediately — naturally fail-fast.

### RC detection in teardown
```go
rcFile := ""
if resolved, rerr := resolveEnv(cfg); rerr == nil {
    if shell := resolved["shell"]; shell != "" {
        // ...detect RC file, check HasSourceLine...
        rcFile = sc.RCFile
    }
}
// if resolveEnv fails (env.yaml absent), rcFile stays "" → skip RC stripping
```

### Test isolation pattern
Tests that exercise setup/teardown use `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` to point config/env paths at a fresh temp dir. The `run()` helper in `main_test.go` is used for all command tests.

### Existing helpers available in package main
- `runPipeline(cfg, dryRun bool)` — full pipeline run (walk → filter → order → act), returns link plan
- `buildActOptions(cfg, dryRun bool)` — builds ActOptions for pipeline.Act calls
- `resolveEnv(cfg)` — returns resolved env map
- `fileExists(path string) bool` — defined in teardown_cmd.go, available to all package main files

---

## Files to create/modify

| File | Action |
|------|--------|
| `internal/setup/shell.go` | Add `RemoveSourceLine` |
| `internal/setup/shell_test.go` | Create — tests for `RemoveSourceLine` |
| `cmd/dotd/init_cmd.go` | Rewrite (scaffold-only) |
| `cmd/dotd/setup_cmd.go` | Create |
| `cmd/dotd/teardown_cmd.go` | Create |
| `cmd/dotd/unapply_cmd.go` | Create |
| `cmd/dotd/main.go` | Register 3 new commands |
| `cmd/dotd/main_test.go` | Add tests for all new commands |

---

## Full plan location

`docs/superpowers/plans/2026-05-23-lifecycle-commands.md` — complete, with all code, test code, and step-by-step instructions.

---

## What NOT to do

- Do not touch PR #66 or #67 — they are separate branches
- Do not commit directly to `main`
- `dotd init` no longer writes config.yaml, env.yaml, or RC line — `dotd setup` does that
- No `deinit` command — `.dagger` files are the user's, manual deletion is correct
- `dotd setup` does NOT touch the shell RC file — deferred to future work
- `dotd teardown` does NOT remove symlinks or `.dagger` files — `dotd unapply` handles symlinks
