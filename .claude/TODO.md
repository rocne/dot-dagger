# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## ✅ DONE — Kill the global `link_root` / `~` overload (pure-XDG roots model)

**Status:** ✅ SHIPPED 2026-06-14 on branch `feature/claude-roots-model` (7-task
plan executed subagent-driven). Default + integration test suites green, vet
clean. Implements `docs/superpowers/specs/2026-06-13-roots-model-design.md`.

Delivered: tokens `~`=$HOME, `$bin`=`($XDG_BIN_HOME ?: ~/.local/bin)/dot-dagger`,
`$config`=$XDG_CONFIG_HOME — all knob-less. Removed every path knob
(link_root/bin_dir/generated_dir/init_file + their flags/envs). config.yaml→
{dotfiles}. Added anchor-token validation + `dotd paths` view.
`--config`→`--dotd-config`, `--env-file`→`--dotd-env`. No backward compat.

Deferred follow-ups (non-blocking): `internal/setup/shell.go` fish branch calls
`ecosystem.XdgConfigHome()` directly instead of receiving `cfg.configDir` (thread
a `configDir` param through `DetectShellConfig`); same file has stale
`cfg.linkRoot`→`cfg.home` doc comments. e2e shell scripts converted but only
CI-verifiable (no local Docker) — watch the first CI run.

Notes below are the ORIGINAL brainstorm — SUPERSEDED (e.g. the `config_dir` knob
was rejected). Kept only for archival context.

### Current state (what the code does today)
- There is ONE global `link_root` knob: `--link-root` flag, `DOTD_LINK_ROOT`
  env, `config.yaml` `link_root` field, and a `setup` wizard prompt
  ("Link root — Home directory used for ~ expansion").
- It is consumed as `homeDir` in `internal/pipeline/act.go` `expandDest`, so
  **`~` expands against `link_root`, not `$HOME`**. Set `link_root: ~/.config`
  and `~/.zshrc` resolves to `/Users/you/.config/.zshrc`.
- `ecosystem.DefaultLinkRoot()` returns `$HOME`, so the *default* is fine —
  but the value is overridable and the setup wizard invites users to override
  it, and existing configs may carry a non-`$HOME` value (the user's does:
  `~/.config`).
- The same `cfg.linkRoot` is also passed as "home" to shell-RC detection in
  `init`/`teardown` (`setup.DetectShellConfig`) and to `adopt` — all of which
  actually just want the real `$HOME`.
- **Tests abuse this:** e2e scripts and integration tests pass
  `--link-root /home/e2e` (or a temp dir) to redirect `~` into a fake home.
  i.e. tests trigger an alternate code path instead of configuring the env.

### What the user objects to (root complaint)
- `~` must ALWAYS mean `$HOME`. Full stop. Nothing should reconfigure what `~`
  means. The current design "changes the meaning of `~` in some contexts,"
  which is wrong.
- "There is no link root." The global link_root concept should not exist.
- Path semantics should be the universal convention: `~` = `$HOME`,
  `/abs` = absolute, `relative` = relative (left as-authored, not silently
  reinterpreted).
- Tests should **configure the environment** (set `$HOME`), NOT pass a magic
  parameter that flips behavior in the code.
- Frustration that this has been raised/corrected before and regressed.

### Agreed decisions (from 2026-06-13 brainstorming)
1. **Scope = NARROW.** Kill the global link_root; make `~` always `$HOME`; add a
   configurable "config root"; fix tests to use the environment. Do NOT do a
   broad rationalization, do NOT rename the per-node `.dagger link_root:` key,
   do NOT touch bin dir behavior.
2. **"Config root" = the target base your CONFIG dotfiles link INTO** (today the
   `config/` convention dir hardcodes `~/.config`). The user wants this to be a
   real, configurable anchor — NOT dot-dagger's own config.yaml location.
3. **Config root gets a FULL knob like `bin_dir`:** new `config_root`
   `config.yaml` field + `--config-root` flag + `DOTD_CONFIG_ROOT` env, default
   `$XDG_CONFIG_HOME` (≈ `~/.config`). (Noted risk: naming proximity to
   `--config` = dot-dagger's own config file; keep help text crisp.)

### Proposed fix — the three-anchor model (section 1, approved-pending)
Split the one overloaded knob into three independent link-target anchors, each
referenced by a token in a `.dagger` `link_root:` value:

| Token | Resolves to | Configurable | Default |
|-------|-------------|--------------|---------|
| `~` | `$HOME` | NO, never | real `$HOME` via `os.UserHomeDir()` |
| `~bin` | bin dir | yes (exists) | `~/.local/bin/dot-dagger` |
| `~config` | config root (NEW) | yes (new knob) | `$XDG_CONFIG_HOME` |

- Delete `--link-root`, `DOTD_LINK_ROOT`, `config.yaml link_root`, the setup
  "Link root" prompt, and `link_root`'s role in `~` expansion.
- The `config/` scaffold role changes from `link_root: "~/.config"` to
  `link_root: "~config"` so scaffolded config dirs follow XDG. Existing
  `.dagger` files that hardcode `~/.config` keep working (= `$HOME/.config`),
  just not XDG-aware unless edited — acceptable under narrow scope.

### Still to design (sections 2–5 not yet presented)
- **Config surface + resolution:** add `config_root` to `internal/config`
  schema, `Keys`, get/set; resolve in `cmd/dotd/main.go resolvePaths` via
  `ecosystem.ResolvePath(... DOTD_CONFIG_ROOT, toolCfg.ConfigRoot, default=
  XdgConfigHome)`; add `--config-root` flag + `pathFlagOwners` entry.
- **Expansion semantics:** add `~config` branch to `expandDest`
  (`internal/pipeline/act.go`), alongside `~bin`; thread `ConfigRoot` through
  `ActOptions` and `ValidateNodes`. Keep the pipeline a PURE function — anchors
  (HomeDir=real $HOME, BinDir, ConfigRoot) are resolved at the cmd layer and
  injected, NOT looked up inside the pipeline.
- **`$HOME` consumers:** introduce a single canonical `cfg.home` (real `$HOME`,
  honors `$HOME` env so tests control it) and use it for `buildActOptions`,
  `ValidateNodes`, init/teardown `DetectShellConfig`, adopt.
- **Migration (IMPORTANT — strict decode):** `internal/config` uses
  `dec.KnownFields(true)`, so simply removing the `LinkRoot` struct field makes
  every existing `config.yaml` containing `link_root:` FAIL to load (incl. the
  user's). Need a backward-compatible removal: keep a hidden deprecated
  `LinkRoot` field (omitempty) tolerated on decode but absent from `Keys`/
  get/set, zeroed on load so it's dropped on next `Save`; warn once if present.
- **Test strategy:** replace every `--link-root` use with environment config —
  e2e Docker scripts `export HOME=/home/e2e` (and set `XDG_CONFIG_HOME` or
  `--config-root` for config-root tests); Go tests `t.Setenv("HOME", tmp)`.
  ~15 e2e scripts + integration_test.go + main_test.go affected. Recall the
  audit lesson: tests ENCODE old behavior — grep `*_test.go` and
  `test/e2e/*.sh` for `--link-root`/`link-root`/`LinkRoot` first.

### Blast radius (files)
`internal/config/config.go`, `internal/ecosystem/ecosystem.go`
(DefaultLinkRoot → expose `Home()`/config-root default), `internal/pipeline/
act.go` + `actions.go` (+ `ActOptions`), `cmd/dotd/main.go` (flags, resolvePaths,
pathFlagOwners, buildActOptions, cfg fields), `cmd/dotd/setup_cmd.go` (drop
prompt, add config_root prompt?), `cmd/dotd/init_cmd.go` (scaffold role, home
usage), `cmd/dotd/teardown_cmd.go`, `cmd/dotd/adopt.go`, `internal/adopter`,
docs (`docs/reference/dotd.md`, concepts, spec `symlinks.md`/`cli.md`/`env.md`),
tests (e2e scripts + integration/main tests).

### Resume point
Re-enter brainstorming; section 1 (anchor model) is agreed; present sections
2–5 (config surface, expansion, migration, tests) for approval, then write the
design doc to `docs/superpowers/specs/2026-06-XX-roots-model-design.md`, user
review, then writing-plans. NO CODE until the plan is reviewed.

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

- [x] cmd/dotd: drop error return on `buildActOptions` (always nil).
- [x] cmd/dotd: inline `dotcfg.DefaultPath()`; sole caller now uses `ecosystem.DefaultConfigFile()` directly.
- [x] cmd/dotd test helper: `runWithStdin(t, io.Reader, args...)` added; interactive-prompt tests no longer re-wire cobra.

## UX / Help

- [ ] `dotd concepts`: add sub-topic routing (`dotd concepts when`, `dotd concepts env`, etc.) once the flat version is validated with users

## Git / CI Infrastructure

- [x] Multi-distro integration testing via Docker — Ubuntu e2e done (PRs #77–78, v0.2.34). Three tests: binary, installer, combined. Failure opens GH issue. Fedora deferred.
- [ ] **Go public** note: Done. `install.sh` now curl-only (PR #76).
