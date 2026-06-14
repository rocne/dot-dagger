# Design: Anchor-token routes, pure-XDG (kill all path knobs)

**Date:** 2026-06-13 (locked 2026-06-14)
**Status:** Approved (design locked), pending implementation plan
**Scope:** Replace the global `link_root` overload with three knob-less anchor
tokens (`~`, `$bin`, `$config`); remove **every** path-route and tool-output
knob in favor of pure environment (XDG) resolution; add anchor-token validation
and a `dotd paths` resolved-view; rename the tool's own state-file flags. No
backward compatibility.

---

## Problem

dot-dagger had **two unrelated things both called "link_root"**:

1. **Per-node `link_root:` key** (in each `.dagger` file) — the legitimate,
   abstract linking mechanism. Each convention dir declares where its files link
   to via a token. This is good and stays.
2. **The global `link_root` knob** (`--link-root` / `DOTD_LINK_ROOT` /
   `config.yaml link_root` / `setup` prompt) — flowed into `expandDest` as
   `HomeDir`, i.e. **it redefined what `~` expands to.** Set `link_root: ~/.config`
   and `~/.zshrc` resolved to `…/.config/.zshrc`. The overload, now deleted.

**Root objection:** `~` must ALWAYS mean `$HOME`; nothing should reconfigure it.

A deeper realization followed: **dot-dagger should put files where the system
already expects them, and let the system (XDG) decide those locations.** Home is
`$HOME`. Config root is `$XDG_CONFIG_HOME`. Managed bins go under the conventional
`~/.local/bin`. Tool-internal output goes under `$XDG_DATA_HOME`. None of these
needs a dot-dagger-specific override knob — the environment is the canonical
source, exactly as `$HOME` is. Per-tool path knobs are redundant with the XDG
environment and add surface without value.

---

## Design

### Anchor tokens — knob-less, env-resolved

A node's `link_root:` value (or an explicit link `dest`) may be an anchor token,
an absolute path, or a relative path (left as-authored). Three tokens exist:

| Token | Resolves to | Configurable | Source |
|-------|-------------|--------------|--------|
| `~` | `$HOME` | no | `os.UserHomeDir()` |
| `$bin` | `($XDG_BIN_HOME ?: ~/.local/bin)/dot-dagger`, added to `PATH` | no | XDG env |
| `$config` | `$XDG_CONFIG_HOME ?: ~/.config` | no | XDG env |

None is a dot-dagger knob: no flag, no `DOTD_*`, no config field. Relocation, when
wanted, is done the universal way — set `$XDG_BIN_HOME` / `$XDG_CONFIG_HOME`.

**Why `$bin` is namespaced but `$config` is not.** `PATH` is a *search list* — the
shell tries every directory on it, so a dedicated `…/dot-dagger` subdir added to
`PATH` works identically to bins in the bare dir, while isolating dot-dagger's
links from unrelated binaries. `$XDG_CONFIG_HOME` is a *direct lookup root* —
apps read exactly `~/.config/<app>`, so configs cannot be namespaced. The
asymmetry reflects search-path vs direct-lookup, not an inconsistency. (`$bin`'s
namespaced layout is unchanged from today's behavior; the only change is it now
honors `$XDG_BIN_HOME` and loses its knob.)

#### Token syntax rationale

`~` is reserved for the one fixed, universal anchor (`$HOME`); only `~` / `~/x`
is recognized. The configurable-by-environment routes use `$`-prefixed tokens
(`$bin`, `$config`): they read as shell-variable-style placeholders, they avoid
the shell `~name` ("home of user name") collision the old `~bin` masqueraded as,
and `$` is not special in YAML (nothing shell-evaluates these values — Go reads
the YAML and `expandDest` resolves the token).

### Anchor token validation

`expandDest` returns any unmatched path unchanged, so without validation a typo
(`$conifg`, `~bin`, `$HOME`) would be treated as a **literal path** and silently
linked to a garbage location. `validateNode` (in `internal/pipeline/actions.go`,
already called by `ValidateNodes`) gains a pure-syntax check applied to both the
per-node `link_root:` value and every explicit link `dest`:

- begins with `~`: valid only as `~` or `~/…`
- begins with `$`: valid only as `$bin`/`$config` or `$bin/…`/`$config/…`
- no leading sigil (absolute/relative): always allowed
- anything else: hard error

Error: `unknown anchor token "$conifg" — valid anchors are ~, $bin, $config`.
Framed purely as a validity check against the current token set — no reference to
prior syntax, renames, or migration. `~`/`$bin`/`$config` are presented as the
only tokens that have ever existed.

### Tool-internal outputs — knob-less

Neither is a link route; both are dot-dagger-managed output the user never names.
Non-configurable, under `$XDG_DATA_HOME`:

| Output | Resolves to |
|--------|-------------|
| generated staging | `$XDG_DATA_HOME/dot-dagger/generated` |
| init.sh | `$XDG_DATA_HOME/dot-dagger/init.sh` |

The generated dir is a **staging area**: compose targets assemble fragments into
it, then the staged file is the *source* of the node's link/source actions
(linked out to `$config`/`$bin`/`~`, or sourced via init.sh). The user only ever
interacts with the final symlink or the source line.

### Resolution & purity

Every anchor and internal path is resolved at the **cmd layer** from `ecosystem`
accessors (no flag/env/config-field tiers — pure env-derived) and injected into
the pipeline. The pipeline stays a pure function: it receives `HomeDir`,
`BinDir`, `ConfigDir`, `GeneratedDir` in `ActOptions` and never reads the
environment itself.

`ecosystem` accessors (the canonical, sole resolvers — no "Default" prefix since
there is no override tier):
- `Home()` → `os.UserHomeDir()` (replaces `DefaultLinkRoot`)
- `BinDir()` → `filepath.Join(XdgBinHome(), Name)` (new `XdgBinHome()`:
  `$XDG_BIN_HOME` if absolute, else `~/.local/bin`)
- `ConfigDir()` → `XdgConfigHome()` (exists)
- `GeneratedDir()` → `$XDG_DATA_HOME/dot-dagger/generated` (exists as
  `DefaultGeneratedDir`)
- `InitFile()` → `$XDG_DATA_HOME/dot-dagger/init.sh` (exists as `DefaultInitFile`)

### Flag namespace (post-change)

Drop all path-route / output knobs entirely (flags + `DOTD_*` envs + config
fields): `--link-root`, `--bin-dir`, `--generated-dir`, `--init-file`. (No
`--config-dir` is ever added.)

The tool's own state-file flags are renamed to a `--dotd-` prefix — marking them
as dot-dagger-internal and deprioritizing them (rare power-user use: pointing at
a non-default config/env for testing or isolated setups). These keep their full
flag + env override because that override is genuinely load-bearing:

| Concept | Flag | Env |
|---------|------|-----|
| Tool's own config.yaml | `--dotd-config` *(was `--config`)* | `DOTD_CONFIG_FILE` |
| Tool's own env.yaml | `--dotd-env` *(was `--env-file`)* | `DOTD_ENV_FILE` |

Surviving flags overall: `--files`/`-f` (dotfiles repo), `--dotd-config`,
`--dotd-env`, `--env`, `--dry-run`, `--force`, log flags.

### `dotd paths` — resolved-view (introspection)

Removing the path config fields removes the old `dotd config get bin_dir`-style
introspection. To keep resolution transparent (not opaque magic), add a
read-only `dotd paths` that prints where every anchor / internal path resolves on
this machine:

```
home       /home/u
$bin       /home/u/.local/bin/dot-dagger
$config    /home/u/.config
generated  /home/u/.local/share/dot-dagger/generated
init.sh    /home/u/.local/share/dot-dagger/init.sh
dotfiles   /home/u/dotfiles
```

`--json` supported; data → stdout, errors → stderr (cli-ux conventions).

### config.yaml shrinks

After dropping `link_root`/`bin_dir`/`generated_dir`, the only remaining field is
`dotfiles`. config.yaml's sole job becomes persisting the dotfiles repo path
(which also resolves via `$DOTD_FILES` → `$DOTFILES` → cwd). Whether config.yaml
still earns its keep is a *later* question — out of scope here; keep it.

### setup wizard shrinks

With bin/config/generated/link-root prompts gone, `setup` reduces to: confirm the
dotfiles repo path, write config.yaml (`dotfiles` only), write env.yaml if absent,
and offer to add the source line to the shell RC. Still useful (onboarding +
shell wiring), just lean.

---

## Migration — no backward compatibility

There are no users; breaking changes are acceptable; no migration notes, no
back-hinting.

**(a) config.yaml fields.** Delete `LinkRoot`, `BinDir`, `GeneratedDir` struct
fields, their `Key*` entries, and get/set cases (leaving only `Dotfiles`). Keep
`dec.KnownFields(true)` strict — a stale `link_root:`/`bin_dir:`/`generated_dir:`
in any config.yaml then fails to load as a plain unknown field. **No special-case
handling or detection** for the removed fields; they are treated as if they never
existed.

**(b) Flags.** Remove `--link-root`/`--bin-dir`/`--generated-dir`/`--init-file`
and their `DOTD_*` envs. Rename `--config`→`--dotd-config`,
`--env-file`→`--dotd-env` (no aliases). Targeted edits at flag registrations and
`pathFlagOwners`.

**(c) Tests → environment configuration.** Replace every `--link-root` (and any
`--bin-dir`/`--init-file`/`--generated-dir`) usage with environment config:
- Go tests: `t.Setenv("HOME", tmp)`, and `t.Setenv("XDG_CONFIG_HOME", …)` /
  `t.Setenv("XDG_BIN_HOME", …)` / `t.Setenv("XDG_DATA_HOME", …)` where a test
  exercises those routes.
- e2e Docker scripts: `export HOME=/home/e2e` (+ `XDG_*` as needed).
- **First step (audit lesson):** grep `*_test.go` and `test/e2e/*.sh` for
  `--link-root`/`link-root`/`LinkRoot`/`bin-dir`/`init-file`/`generated-dir` and
  enumerate every site before editing — tests encode old behavior.

**(d) Token spelling in fixtures.** `~bin` → `$bin` in scaffold output and
testdata `.dagger` files. Config scaffold `link_root: "~/.config"` → `"$config"`.

---

## Blast radius (files)

- `internal/pipeline/act.go` (+ `actions.go`) — `expandDest` `$bin`/`$config`/`~`
  branches; `BinPrefix="$bin"`, `ConfigPrefix="$config"`; `ActOptions.ConfigDir`;
  `validateNode` anchor-token validation.
- `internal/ecosystem/ecosystem.go` — `Home()`, `XdgBinHome()`, `BinDir()`,
  `ConfigDir()`; rename/retain `GeneratedDir()`/`InitFile()`; remove
  `DefaultLinkRoot`, the knob-style `DefaultBinDir` flavor.
- `internal/config/config.go` — strip to `Dotfiles` only.
- `cmd/dotd/main.go` — remove path-route flags + cfg fields + their resolution;
  rename `--dotd-config`/`--dotd-env`; resolve anchors/outputs from accessors;
  inject `ConfigDir` into `ActOptions`; `pathFlagOwners` cleanup.
- `cmd/dotd/setup_cmd.go` — drop bin/generated/link-root prompts.
- `cmd/dotd/init_cmd.go` — scaffold `$bin`/`$config`; `Home()`/`InitFile()`.
- `cmd/dotd/teardown_cmd.go`, `cmd/dotd/adopt.go` (+ `internal/adopter`) —
  accessors; adopt gains `ConfigDir`.
- `cmd/dotd/config_cmd.go` — help examples; `config show`/`get`/`set` now only
  `dotfiles`.
- **New:** `cmd/dotd/paths_cmd.go` — `dotd paths` resolved-view.
- Docs: `README.md`, `docs/reference/dotd.md`/`dagger.md`/`annotations.md`/
  `env-yaml.md`, concepts, spec `symlinks.md`/`cli.md`/`env.md`. (CI `.github/`
  `--config` refs are GoReleaser's own flag — unchanged. No committed shell
  completions.)
- Tests + fixtures: e2e scripts + `integration_test.go` + `main_test.go`;
  testdata `.dagger` (`~bin`→`$bin`).

---

## Out of scope (explicitly)

- Renaming the per-node `.dagger link_root:` key (stays).
- Removing `config.yaml` entirely (its `dotfiles`-only future is a later call).
- Tilde-in-config-value expansion (`ResolvePath` doesn't expand `~`; pre-existing
  and now near-moot since the only path field left is `dotfiles`; CLI args are
  shell-expanded anyway).
- Per-tool (non-global) path relocation — intentionally dropped; use `$XDG_*`.
