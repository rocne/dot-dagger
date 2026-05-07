# dot-dagger v2 Redesign

**Date:** 2026-05-07
**Status:** Draft
**Scope:** Ground-up redesign of interface, annotation system, config format, and CLI

---

## 1. Goals

Total shell management system for multiple OS, system, and context targets. v2 questions everything — annotations, config format, CLI, vocabulary — with no migration burden (no existing users).

Primary target: single user managing their own dotfiles across machines and contexts. Designed to accommodate expansion later (teams, sharing, distribution) without committing to it now.

---

## 2. Core Model

### Nodes

Every file or directory in the dotfiles repo is a **node**. Nodes form a DAG, identified by logical name.

**Logical name derivation** — per path component:
1. Strip leading `nosync-`
2. Strip leading `dot-`
3. Strip file extension from final component only

```
tmux/shellrc/helpers.sh        → tmux.shellrc.helpers
conf/dot-tmux.conf             → conf.tmux.conf
nosync-work/shellrc/aliases.sh → work.shellrc.aliases
```

### Subtype Hierarchy

```
BasicNode
  when       — predicate expression, ANDed with parent defaults
  link_root  — base path for link destinations
  actions    — ordered list of actions to apply

NamedNode : BasicNode
  name       — override logical name (for @after references and variant patterns)

ComposableNode : NamedNode
  defaults   — BasicNode cascaded to all children
  files      — dict[path → NamedNode] for files that can't carry annotations
  composition
    enabled  — mark this dir as a compose target
```

### Variant Pattern

Two files that represent the same logical unit under different conditions:

```bash
# dot-gitconfig-work
# @name gitconfig
# @when context=work
# @link(~/.gitconfig)

# dot-gitconfig-personal
# @name gitconfig
# @when context=personal
# @link(~/.gitconfig)
```

Same `@name`, mutually exclusive `@when`. Engine enforces exactly one active at a time. Two active nodes with the same logical name = hard error.

### Compose Targets

A directory marked `composition: enabled: true` in `.dagger` is a compose target. Its children are **fragments** — content only, assembled in DAG order into a single generated file. The generated file is then handled by subsequent actions (link, source).

Fragments support `@when` and `@after`. All other action annotations on fragments are errors.

---

## 3. Annotations

Declared as comment annotations at the top of a file.

### Scanner Rules

1. If first line is a shebang (`#!`), skip it
2. Read contiguous lines starting with `#` or `//`
3. First non-comment, non-blank line stops the scan
4. Non-`@` comment lines ignored without stopping scan

### Syntax

Unified function-call style:

```bash
# @when(os=macos)
# @when(context=work)
# @after(shellrc.base)
# @after(shellrc/)
# @name(my.logical.name)
# @link(~/.tmux.conf)
# @link(relative/path)
# @source
# @no-source
# @disable
# @require(package-name)
# @request(package-name)
```

Multiple `@when` lines are ANDed. `@when` and `@after` are file-level only (not per-action).

### Supported Annotations

| Annotation | Purpose |
|-----------|---------|
| `@when(expr)` | Inclusion predicate. Multiple lines ANDed. |
| `@after(ref)` | DAG ordering — logical name or path prefix ending in `/` |
| `@name(logical-name)` | Override full logical name |
| `@link(dest)` | Symlink this file to dest |
| `@source` | Include in init.sh sourcing |
| `@no-source` | Exclude from init.sh sourcing |
| `@disable` | Exclude from all processing |
| `@require(pkg)` | Hard package dependency |
| `@request(pkg)` | Soft package dependency |

### Actions

Actions declare what to do with a file or directory. Multiple actions applied in declaration order. `@link`, `@source`, `@no-source` are annotation shorthands for their action equivalents.

| Action | Declared via | Applies to | Description |
|--------|-------------|-----------|-------------|
| `compose` | `.dagger` only | directories | Assemble fragments into generated file |
| `link(dest)` | `@link(dest)` or `.dagger` | files, directories | Symlink to dest |
| `source` | `@source` or `.dagger` | files, directories | Include in init.sh |
| `no-source` | `@no-source` or `.dagger` | files, directories | Exclude from init.sh |

`compose` on a file is a hard error. `link`/`source` on a compose target operate on the generated output. `link` without dest argument is a hard error.

---

## 4. `.dagger` Config File

Per-directory metadata for files that cannot carry annotations, and for directory-level settings.

**File name:** `.dagger` (read aloud: "dot dagger")

### Schema

```yaml
# ComposableNode — full form (on a directory)
when: os=macos
link_root: ~/relative/path     # relative extends parent cascade; absolute replaces
name: override.name
actions:
  - link(dest)
  - source

defaults:                      # BasicNode — cascades to all children
  when: context=work
  link_root: ~/some/path
  actions:
    - no-source

composition:
  enabled: true               # mark as compose target

files:                        # dict[path → NamedNode]
  settings.json:
    when: os=macos
    name: nvim.settings
    actions:
      - link(settings.json)
  dot-gitconfig-work:
    when: context=work
    actions:
      - link(~/.gitconfig)
```

### Link Root Cascading

`link_root` cascades through the directory tree:
- **Relative path** — extends parent's link_root (`hey` under `~/blah` → `~/blah/hey`)
- **Absolute path** (`/` or `~/`) — replaces cascade entirely

### `dotd init` Pre-population

`dotd init` scaffolds `.dagger` files to mark convention dirs. No engine-level recognition of directory names — `.dagger` is what makes a dir behave as `shellrc/`, `conf/`, or `bin/`. Users can name dirs anything.

Example scaffolded `shellrc/.dagger`:
```yaml
defaults:
  actions:
    - source
```

---

## 5. Pipeline

Five stages, run in order:

```
env → walk → filter → order → act
```

**env** — load `config.yaml` and `env.yaml`, evaluate shell expressions, resolve env keys. Prompt for missing keys (TTY) or halt (non-interactive).

**walk** — traverse dotfiles repo, parse annotations and `.dagger` files, build raw node list.

**filter** — evaluate `@when` predicates against resolved env. Discard non-matching nodes. Compose dirs with zero active children become inactive.

**order** — Kahn's topological sort on `@after` edges, alphabetical tie-break by logical name. Compose targets use isolated per-target sub-DAG.

**act** — execute actions in declaration order per node:
- `link(dest)` → create/verify symlink
- `source` → add entry to init.sh
- `no-source` → suppress convention default source
- `compose` → concat active fragments in DAG order → write generated file → register synthetic node for downstream actions

`dotd apply` runs all five. `dotd check` runs env→walk→filter→order, validates act outputs without writing.

**Conflict detection** (during order):
- Same logical name from two active nodes → hard error
- Same link dest from two active nodes → hard error

---

## 6. CLI

```
dotd apply                       # full pipeline
dotd check                       # validate without writing
dotd init                        # interactive wizard — scaffold dotfiles repo
dotd list [--active] [--inactive] [--json]
dotd bundle <file> [--output <file>] [--include-env]
                                 # inline DAG deps of <file> into portable script

dotd env show
dotd env get <key>
dotd env set <key> <value>
dotd env edit

dotd config show
dotd config get <key>
dotd config set <key> <value>
dotd config edit

dotd help --all                  # show all commands including hidden internals
```

### Hidden Internal Commands

Not shown in default help. Revealed by `dotd help --all`. Intended for use in `env.yaml`:

```
dotd get-os        # normalized OS string (macos, linux, etc.)
dotd get-hostname  # system hostname
```

Users can call these in `env.yaml` shell expressions or anywhere in shell.

### Global Flags

`--dotfiles <path>`, `--link-root <path>`, `--dry-run`, `--generated-dir <path>`

`dotd apply` and `dotd check` also accept `--env key=val,key2=val2` to override env at run time.

---

## 7. Config / Env System

Both files live in `~/.config/dot-dagger/`.

### `config.yaml` — tool preferences, machine-stable

```yaml
dotfiles: ~/dotfiles
bin_dir: ~/bin
generated_dir: ~/.config/dot-dagger/generated
link_root: ~
```

### `env.yaml` — predicate variables, context-dependent

Supports shell interpolation via `$(...)` — evaluated at pipeline start.

```yaml
os: $(dotd get-os)
hostname: $(dotd get-hostname)
context: work
my_gpu: $(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null || echo none)
```

`dotd init` pre-populates `os` and `hostname`. Users add custom keys for their own predicates.

If a command fails (non-zero exit or empty output) → treated as missing key → prompt or halt.

### Env Resolution Order (highest → lowest)

1. `--env` CLI flags
2. Shell env vars (e.g. `DOTD_CONTEXT=work`)
3. `env.yaml` (static values and evaluated `$(...)`)
4. Prompt (TTY) or halt (non-interactive)

---

## 8. Out of Scope (v2)

- Shorthand annotations (`@bin`, `@config`) — deferred until pattern is clear from real usage
- Multi-user / team dotfiles sharing
- Package installation (manifests may still be parsed, but install is out of scope)
- Migration tooling from v1
