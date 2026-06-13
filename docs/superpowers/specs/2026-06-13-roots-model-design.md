# Design: Kill the global `link_root`, add a `config_dir` route

**Date:** 2026-06-13
**Status:** Approved (brainstorming), pending implementation plan
**Scope:** Narrow — remove the global `link_root` overload, make `~` always
`$HOME`, add a configurable config route, rename the tool's own state-file flags,
and convert tests to environment configuration. No backward compatibility.

---

## Problem

dot-dagger has **two unrelated things both called "link_root"**, and conflating
them is the whole bug:

1. **Per-node `link_root:` key** (in each `.dagger` file) — the legitimate,
   abstract linking mechanism. Each convention dir declares where its files link
   to via a token: `~` (`$HOME`), `$bin` (bin dir). This is good and stays.
   (Token syntax updated from `~bin` → `$bin`; see "Token syntax" below.)

2. **The global `cfg.linkRoot` knob** (`--link-root` flag, `DOTD_LINK_ROOT` env,
   `config.yaml link_root` field, `setup` wizard prompt) — flows into
   `expandDest` as `HomeDir`, i.e. **it redefines what `~` expands to.** Set
   `link_root: ~/.config` and `~/.zshrc` resolves to `…/.config/.zshrc`. This is
   the overload to delete.

**Root objection:** `~` must ALWAYS mean `$HOME`. Nothing should reconfigure it.
"There is no link root." Tests make it worse by passing `--link-root /home/e2e`
to fake a home — triggering an alternate code path instead of configuring the
environment.

**Why the global knob is pure misfeature:** all 7 consumers of `cfg.linkRoot`
actually want the *real* `$HOME` — `buildActOptions`/`ValidateNodes` (`~`
expansion), `init`/`teardown` `DetectShellConfig` + `AppendSourceLine`, `adopt`,
and the `setup` prompt (which we delete). Not one benefits from a configurable
home. The knob exists only to (a) redefine `~` (the bug) and (b) fake `$HOME` in
tests (the abuse).

**Why it was built this way (context check):** the original instinct — "configs
get symlinked, bins go on PATH" — is *preserved* by the per-node `~config` +
`~bin` anchors. The global `link_root` was never what made config-linking work;
`config/.dagger` already carries its own route. So removing it does not regress
the original rationale.

---

## Design

### The three-anchor model

Linking stays pure and abstract: a node's `link_root:` value may be any anchor
token, an absolute path, or a relative path (left as-authored). Three anchor
tokens exist; **`$config` is just another token, not a magic special-case of
dirs named `config/`**.

| Token | Resolves to | Configurable | Default |
|-------|-------------|--------------|---------|
| `~` | `$HOME` | **never** | `os.UserHomeDir()` (real `$HOME`) |
| `$bin` | bin dir | yes (exists) | `~/.local/bin/dot-dagger` |
| `$config` | config dir | yes (**new**) | `$XDG_CONFIG_HOME` (≈ `~/.config`) |

#### Token syntax rationale

`~` is reserved for the **one fixed, universal, non-configurable** anchor:
`$HOME`. It is kept because it is the universal convention and, with the global
knob gone, finally means exactly `$HOME`. Only the bare `~` / `~/x` form is
recognized.

The **configurable** routes use a `$`-prefixed, shell-variable-style token
(`$bin`, `$config`). Rationale: (1) they are literally backed by env vars
(`DOTD_BIN_DIR`, `DOTD_CONFIG_DIR`), so "variable" reads true; (2) `~name` in
real shells means "home dir of user `name`" — so the old `~bin`/`~config` tokens
masqueraded as that syntax; `$`-tokens avoid the collision; (3) `$` is not
special in YAML and nothing shell-evaluates these values (Go reads the YAML and
`expandDest` resolves the token), so there is no interpolation hazard. The
distinction also encodes meaning: `~` = fixed home, `$x` = resolved named root.

This changes the **existing** bin token `~bin` → `$bin` (syntax only, no behavior
change). Acceptable under "no backward compatibility."

### Flag / knob namespace (post-change)

A clean split: `--dotd-*` flags point at dot-dagger's **own** state files (rare,
power-user); `--*-dir` flags are **link-target routes** (the common surface).

| Concept | Flag | Env | config.yaml field | Default |
|---------|------|-----|-------------------|---------|
| Tool's own config.yaml | `--dotd-config` *(was `--config`)* | `DOTD_CONFIG_FILE` | — | `$XDG_CONFIG_HOME/dot-dagger/config.yaml` |
| Tool's own env.yaml | `--dotd-env` *(was `--env-file`)* | `DOTD_ENV_FILE` | — | `$XDG_CONFIG_HOME/dot-dagger/env.yaml` |
| Bin route | `--bin-dir` | `DOTD_BIN_DIR` | `bin_dir` | `~/.local/bin/dot-dagger` |
| **Config route** | `--config-dir` | `DOTD_CONFIG_DIR` | `config_dir` | `$XDG_CONFIG_HOME` |

Notes:
- `config_dir` is symmetric with `bin_dir` (both `*_dir`). The token is
  `$config` (token ≠ field-suffix is already precedent: `$bin` token vs `bin_dir`
  field).
- The `--dotd-` prefix marks "internal to the tool"; it removes the
  `--config` / `--config-dir` collision. Env vars keep their existing
  `DOTD_CONFIG_FILE` / `DOTD_ENV_FILE` names — the `DOTD_` prefix already does the
  "tool-level" signalling there.
- `--dotd-config`/`--dotd-env`'s real use: pointing at a non-default
  config/env for testing or running isolated setups side by side. Legitimate but
  rare — hence demoted, not deleted.

### Config-route resolution

`config_dir` is a full knob resolved once in `resolvePaths()` via the standard
chain, stored as `cfg.configDir`, injected into `ActOptions`:

```
--config-dir flag → DOTD_CONFIG_DIR env → config.yaml config_dir → $XDG_CONFIG_HOME
```

`ecosystem.XdgConfigHome()` already exists and is the default source. The
pipeline never looks it up — it receives the resolved anchor.

### `~` / `$HOME`: no tracking at all

There is **no** `cfg.home` resolved field, no flag, no env, no config knob, no
resolution chain — nothing to resolve. `os.UserHomeDir()` returns `$HOME`
verbatim on linux/darwin and errors only when `$HOME` is unset (no `/etc/passwd`
fallback — verified, Go 1.26.3). The system/shell configures home; the tool
inherits it.

A single thin accessor `ecosystem.Home()` (wrapping the existing private
`userHome()`) centralizes the one consistent "`$HOME` is not defined" error and
gives callers one name. It is explicitly **not** a canonical-resolution path —
it's a pure env read, the CLAUDE.md "universal convention with no project-specific
override" exception (like `$EDITOR`). `ecosystem.DefaultLinkRoot()` is removed;
`Home()` replaces it.

cmd layer calls `ecosystem.Home()` to inject `HomeDir` into
`ActOptions`/`ValidateNodes` (pipeline stays pure). `init`/`teardown`/`adopt`
call the same accessor.

### Expansion semantics

`expandDest(path, homeDir, binDir, configDir)` gains a third anchor branch:

- `~` / `~/x` → `homeDir` (real `$HOME`)
- `$bin` / `$bin/x` → `binDir`
- `$config` / `$config/x` → `configDir`
- anything else (`/abs`, `relative`) → left as-authored

Whole-token matching (already the discipline for `~` via the `path[1] == '/'`
guard) ensures each token matches only as a whole token or with a `/` suffix
(`$config` is not a prefix-confusion of `$bin`, etc.). `ActOptions` gains
`ConfigDir string`; thread it through `ValidateNodes` and `buildActOptions`. The
pipeline remains a pure function — all three anchors resolved at the cmd layer
and injected, never looked up inside the pipeline.

---

## Migration — no backward compatibility

Breaking changes are acceptable. Remove the bad field and old flag names cleanly;
preserve no remnants.

**(a) Remove `link_root` from config.yaml.** Delete the `LinkRoot` struct field,
its `KeyLinkRoot` entry, and its get/set cases. Keep `dec.KnownFields(true)`
strict — a stale `link_root:` in any existing config.yaml (including the author's
own) will then fail to load with a clean "unknown field" error, prompting the
user to delete the dead line. No hidden field, no warn-once, no compatibility
shim.

**(b) Rename flags in place, no aliases.** `--config` → `--dotd-config`,
`--env-file` → `--dotd-env`; add `--config-dir`. Targeted edits at the flag
registrations and `pathFlagOwners` keys — not blind `sed`, because `"config"`
also appears as the cobra group ID and inside `config_dir`. Order the edits to
avoid partial-string collisions.

**(c) Tests → environment configuration.** Replace every `--link-root` usage with
environment config:
- Go tests: `t.Setenv("HOME", tmp)` instead of `--link-root tmp`.
- e2e Docker scripts: `export HOME=/home/e2e` (plus `export DOTD_CONFIG_DIR=…`
  or `--config-dir` where a test exercises config linking).
- **First implementation step (audit lesson):** grep `*_test.go` and
  `test/e2e/*.sh` for `--link-root` / `link-root` / `LinkRoot` and enumerate
  every site *before* changing code — tests encode the old behavior, so the grep
  is the real blast-radius map.

**Scaffold change.** `init` emits `config/.dagger` with `link_root: "$config"`
(was `~/.config`) and `bin/.dagger` with `link_root: "$bin"` (was `~bin`), so
scaffolded config dirs follow XDG. Existing user `.dagger` files that hardcode
`~/.config` keep working (= `$HOME/.config`), just not XDG-aware until edited.
Existing `.dagger` files using the old `~bin` token must be updated to `$bin`
(no back-compat) — acceptable under narrow scope.

---

## Blast radius (files)

- `internal/config/config.go` — drop `LinkRoot`/`KeyLinkRoot`; add
  `ConfigDir`/`KeyConfigDir` + get/set.
- `internal/ecosystem/ecosystem.go` — remove `DefaultLinkRoot`; add `Home()`
  accessor; `XdgConfigHome` already present for the config-dir default.
- `internal/pipeline/act.go` (+ `actions.go`) — `expandDest` third branch;
  rename `BinPrefix` const `~bin` → `$bin`; add `ConfigPrefix` = `$config`;
  `ActOptions.ConfigDir`.
- `cmd/dotd/main.go` — flag renames (`--dotd-config`, `--dotd-env`), new
  `--config-dir`, drop `--link-root`; `pathFlagOwners` updates; `resolvePaths`
  (drop linkRoot resolve, add configDir resolve); `buildActOptions`/
  `ValidateNodes` use `ecosystem.Home()` + `cfg.configDir`.
- `cmd/dotd/setup_cmd.go` — delete the "Link root" prompt.
- `cmd/dotd/init_cmd.go` — scaffold `~config`; use `ecosystem.Home()`.
- `cmd/dotd/teardown_cmd.go`, `cmd/dotd/adopt.go` — use `ecosystem.Home()`.
- `cmd/dotd/config_cmd.go` — help examples (`link_root` → `config_dir`).
- Docs: `docs/reference/dotd.md`, concepts, spec `symlinks.md`/`cli.md`/`env.md`.
- Tests + fixtures: e2e scripts + `integration_test.go` + `main_test.go`
  (link-root → env config); testdata `.dagger` fixtures using `~bin` →
  `$bin` (`internal/pipeline/testdata`, `cmd/dotd/testdata`, `test/e2e/fixture`).

---

## Out of scope (explicitly)

- Renaming the per-node `.dagger link_root:` key (stays).
- Touching `bin_dir` *behavior* or renaming the field to `bin_root`. (The bin
  *token* spelling does change `~bin` → `$bin` for cross-route consistency; the
  resolution/default/field are untouched.)
- `generated_dir`.
- Any backward-compatibility machinery (config field shims, flag aliases).
