# dot-dagger — Design Specification

**Status:** Draft v1.2
**CLI command:** `dotd`
**Language:** Go
**Architecture:** Separate tool repo; dotfiles repo is pure data

---

## 1. Overview

dot-dagger is a dotfiles composition engine. It walks a dotfiles directory tree, evaluates predicate expressions on each file, resolves a DAG-based sourcing order, generates a single shell init file, and symlinks dotfiles and bin commands into place.

It is intentionally minimal. Package management is a separate tool. The predicate system is the core engine — everything else is convention.

---

## 2. Directory Conventions

Any directory in the dotfiles repo can contain these conventionally-named subdirectories. Conventions apply at any depth.

| Directory | Behavior |
|-----------|----------|
| `scripts/` | Files are sourced into the generated `init.sh` in DAG order |
| `bin/` | Files are symlinked into the managed bin dir |
| `dots/` | Files are symlinked into `$HOME` with `dot-` prefix transformed |

These conventions require zero configuration.

### The `dot-` prefix

Files in `dots/` use `dot-` instead of `.` so they remain visible in the repo. When computing **symlink destinations**, every path component under `dots/` that starts with `dot-` has the prefix replaced with `.`:

```
dots/dot-zshrc                  → ~/.zshrc
dots/dot-config/tmux/tmux.conf  → ~/.config/tmux/tmux.conf
dots/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
```

Path components without the `dot-` prefix are used as-is. Replacement applies at every level of the path, not just the top level.

**Important:** This is distinct from logical name derivation (see §3), where `dot-` is simply stripped entirely. The two transformations serve different purposes:

| Context | Rule | Example |
|---------|------|---------|
| Logical names (DAG) | strip `dot-` entirely | `dot-zshrc` → `zshrc` |
| Symlink destinations | replace `dot-` with `.` | `dot-zshrc` → `.zshrc` |

To opt out of prefix transformation for a specific file, use `@retain-prefix` in the file's annotation block, or declare `retain_prefix: true` in `.dotd.yaml`. The `RetainPrefix` flag applies only to the file's own path component; intermediate directory components are always transformed.

### The `nosync-` prefix

Any file or directory prefixed with `nosync-` is automatically gitignored, fully functional at runtime, and has the prefix stripped from its logical name and symlink destination. Applies at any level.

During `dotd install`, the tool ensures `nosync-*` is present in `.gitignore` before any other operation. This prevents accidental staging of private files on fresh repos.

---

## 3. Logical Names and the DAG

Every file is a node in the DAG, identified by its logical name — a dot-separated path from the dotfiles repo root with the file extension stripped.

### Name derivation

Applied per path component, in order:
1. Strip leading `nosync-`
2. Strip leading `dot-` (after nosync-)
3. Strip file extension from the final component only

```
tmux/scripts/helpers.sh         → tmux.scripts.helpers
scripts/math.sh                 → scripts.math
nosync-work/scripts/aliases.sh  → work.scripts.aliases
dots/dot-config/tmux/tmux.conf  → dots.config.tmux.tmux
nosync-dot-secrets/api.sh       → secrets.api
```

The logical name always reflects the full path. No segments are skipped regardless of whether intermediate directories have `.dotd.yaml` files.

### `@name` — aliasing

`@name` replaces the entire logical name, not just the last segment. It creates a canonical identity for the file independent of its filesystem location:

```bash
# tmux/scripts/tmux-helpers-macos.sh
# @name tmux.scripts.helpers
```

This is the primary tool for variant files — two files that represent the same logical unit under different conditions both declare the same `@name`. The resolver enforces that exactly one is active at a time. Predicates must be mutually exclusive.

### `@after` references

`@after` accepts either a full logical name or a path prefix ending in `/`:

```bash
# @after tmux.scripts.helpers   — specific file by logical name
# @after tmux/                  — all active files under tmux/
# @after tmux/scripts/          — all active files under tmux/scripts/
```

Path-prefix references expand to the set of active files under that path. If no files under that path are active, the dependency is a no-op — never an error.

`@after` is purely an ordering constraint. It does not affect inclusion. A file being inactive does not cause an `@after` referencing it to fail.

### Default ordering

Files with no `@after` declarations are sourced in deterministic alphabetical order by logical name within each topological frontier. This ensures reproducible `init.sh` generation regardless of filesystem discovery order.

### Conflict detection

Two active nodes with the same logical name — whether derived or declared via `@name` — is a conflict error. Predicates must be mutually exclusive for variant files sharing a name.

Two active files both declaring `@symlink` to the same destination is a separate symlink destination conflict — detected independently from logical name conflicts. Conventional `dots/` and `bin/` destination conflicts are detected by the linker, which knows the runtime home directory.

---

## 4. The Predicate System

Every file can declare a `when` condition. A file is active if its effective predicate evaluates to true against the current environment.

A directory-level `when` in `.dotd.yaml` applies to all files in that directory and all subdirectories as a shared default. The effective predicate for any file is:

```
directory_when AND file_when
```

A file with no `when` is implicitly true — active whenever its directory predicate is satisfied.

### Environment keys

| Key | Auto-detected? | Method | Examples |
|-----|---------------|--------|---------|
| `os` | Yes | `runtime.GOOS`, normalized | `macos`, `linux` |
| `distro` | Yes | `/etc/os-release` or `sw_vers` | `ubuntu`, `sequoia` |
| `shell` | Yes | `$SHELL`, basename, lowercased | `zsh`, `bash` |
| `context` | No | Must be set explicitly | `personal`, `work` |

`darwin` and `macos` are treated as aliases — `runtime.GOOS` returns `darwin`, which is normalized to `macos` at detection time.

Custom keys are declared in `config.yaml` with optional `detect`, `cmd`, `default`, and `values` fields.

### Environment resolution precedence

1. `--env key=val` CLI flag
2. Explicit value in `env.yaml`
3. `cmd` output from `config.yaml`
4. `detect` auto-detection
5. `default` static fallback from `config.yaml`
6. Unset — surfaces as error or prompt, never silent

### Missing keys at apply time

If `context` or any other required key is unset when `dotd apply` runs, the tool prompts interactively if a TTY is present, or halts with a clear error in non-interactive mode. Files gated on unset keys are not silently excluded.

### Predicate grammar

```
expr       = or_expr
or_expr    = and_expr (OR and_expr)*
and_expr   = atom (AND atom)*
atom       = "(" expr ")" | call | condition
call       = IDENT "(" IDENT ")"
condition  = KEY "=" VALUE ("," VALUE)*
```

`AND` binds tighter than `OR`. Parentheses override. Comma is same-key OR shorthand. `AND` and `OR` are case-sensitive uppercase keywords.

### Builtin predicate functions

| Function | Meaning |
|----------|---------|
| `exists(binary)` | True if binary is on PATH |

### Multi-line `@when`

Multiple `@when` lines are combined with AND:

```bash
# @when os=macos OR os=linux
# @when context=work
# effective: (os=macos OR os=linux) AND context=work
```

---

## 5. Annotations

Metadata is declared as comment annotations at the top of a file.

### Scanning rules

1. If the first line is a shebang (`#!`), skip it and allow one immediately-following blank line
2. Read contiguous lines starting with `#` (all leading `#` stripped) or `//`
3. Non-`@` comment lines are ignored without stopping the scan
4. Any blank line or non-comment line stops the scan

### Supported annotations

| Annotation | Purpose |
|-----------|---------|
| `@when <expr>` | Inclusion predicate. Multiple lines are ANDed. |
| `@after <ref>` | DAG ordering dependency — logical name or path prefix ending in `/` |
| `@name <logical-name>` | Override the full logical name of this file |
| `@module <n>` | Declare module membership (for organisational tooling) |
| `@symlink <path>` | Symlink this file to an explicit destination (tilde-expanded) |
| `@retain-prefix` | Opt out of `dot-` transformation for this file's last path component |
| custom | Dispatched to registered external handlers |

### `@symlink`

Opts any file into symlinking at an explicit destination. Rarely needed — `dots/` handles the common case. Used for files outside `dots/` that need symlinking, or for dotfiles that need a non-conventional destination.

Files in `dots/` are symlinked by convention — `@symlink` is only needed there to override the default destination. `@symlink` takes precedence over convention in all cases.

### Custom annotations

Unknown annotations are not errors. They are dispatched to registered external handlers declared in `config.yaml`:

```yaml
annotation_handlers:
  requires: dag-pkg procure
```

So `@requires fzf` becomes `dag-pkg procure fzf` at apply time.

In `--dry-run` mode, annotation handlers are not invoked. The tool prints what would be called instead.

If a handler is not installed or fails, the tool warns but continues by default. With `--no-interactive`, failures halt execution.

---

## 6. `.dotd.yaml`

An optional metadata file that can appear in any directory. Its primary purpose is to provide metadata for files that cannot carry annotations — JSON, binary, XML, and other formats without supported comment syntax.

`.dotd.yaml` has three sections:

**`directory`** — properties of the directory node itself. Does not cascade to contents. The `when` field gates traversal of the entire subtree; if false, the directory is not entered at all.

**`defaults`** — values that cascade to all files inside the directory and all subdirectories, ANDed with each file's own annotations.

**`files`** — per-file metadata for specific files that cannot carry annotations.

```yaml
# Properties of this directory node itself
directory:
  when: "os=macos"        # gates traversal — don't enter unless this matches
  retain_prefix: true     # don't transform dot- on this directory's name in symlink paths

# Defaults that cascade to all files inside
defaults:
  when: "context=work"

# Per-file metadata for files that cannot carry annotations
# path: true filename as it exists on disk
# when/after/name/symlink: use logical name semantics, same as annotations
files:
  - path: dot-gitconfig-work
    when: "context=work"
    symlink: ~/.gitconfig

  - path: dot-gitconfig-personal
    when: "context=personal"
    symlink: ~/.gitconfig

  - path: settings.json
    symlink: "~/Library/Application Support/SomeApp/settings.json"
    when: "os=macos"
    retain_prefix: true
```

All sections are optional. A `.dotd.yaml` with just a `defaults.when` is valid. A directory with no `.dotd.yaml` is also perfectly fine.

Declaring a predicate in `.dotd.yaml` for a specific file AND as an annotation in that file is an error. All predicate expressions are validated at load time.

---

## 7. Config Files

### `config.yaml` (committed — repo root)

Declares the environment schema and annotation handlers.

```yaml
env:
  os:     { detect: true }
  distro: { detect: true }
  shell:  { detect: true }
  context:
    detect: false
    values: [personal, work]
  role:
    default: desktop
  host:
    cmd: hostname

annotation_handlers:
  requires: dag-pkg procure
```

### `~/.config/dot-dagger/env.yaml` (not committed — machine local)

```yaml
env:
  context: work
  role: desktop
```

---

## 8. Shell Init Integration

dot-dagger generates a single shell-agnostic init file at `~/.config/dot-dagger/init.sh`. The user adds one line to their shell rc file:

```sh
. ~/.config/dot-dagger/init.sh
```

The init file uses POSIX `.` (not bash `source`) for compatibility with sh, bash, and zsh. Paths are single-quoted using the `'\''` idiom to handle embedded spaces and special characters correctly.

During `dotd install`, the tool checks for this line, notifies if already present, or asks for confirmation before adding it. The tool never modifies an rc file without explicit confirmation.

The init file is a build artifact — not committed, fully regenerated by `dotd apply`. It is written atomically (temp file + rename) so a crash during apply never leaves a partial init.sh.

The managed bin directory is prepended to `PATH` using `${HOME}/...` notation rather than a hardcoded path, so the init.sh is portable across machines with different home directory paths.

### Shell type

All scripts are sourced into a single `init.sh`. Shell-specific inclusion is handled via `@when shell=zsh` predicates. There is no separate per-shell init file.

---

## 9. Symlink Strategy

### `dots/` → `$HOME`

```
dots/dot-zshrc                     → ~/.zshrc
dots/dot-config/tmux/tmux.conf     → ~/.config/tmux/tmux.conf
dots/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
```

Every `dot-` prefix at every path level is replaced with `.`. Non-`dot-` components are used as-is.

### `bin/` → managed bin dir

```
bin/tmux-sessionizer → ~/.local/bin/dot-dagger/tmux-sessionizer
```

The managed bin dir is added to PATH in the generated `init.sh`. It is the only PATH addition dot-dagger makes.

### `@symlink` → explicit destination

Any file anywhere in the repo can declare `@symlink <path>` to be symlinked to an explicit destination. Tilde (`~/`) is expanded to the home directory.

### Ownership

A symlink is owned by dot-dagger if its current target starts with the repo root path. Owned symlinks are updated freely (e.g. when switching between variant files). Foreign symlinks — pointing outside the repo — require `--force`.

### Conflict handling

If a real file exists where dot-dagger expects one of its symlinks, the tool warns and requires `--force` to proceed. It never silently removes or overwrites files it does not own.

### Deactivation cleanup

When a file's predicate no longer matches, `dotd apply` removes its deployed artifacts. Default behaviour for v1 — configurable later via `on_deactivate` setting.

---

## 10. Drift Detection

`dotd status files` compares deployed state to source at runtime. No state file — candidate paths are derived from active nodes in the current environment.

Symlink states reported:

| State | Meaning |
|-------|---------|
| OK | Symlink exists and points to the correct source |
| Missing | Nothing at the expected destination |
| WrongTarget | Symlink exists but points elsewhere |
| Conflict | Real file at the expected destination |

Fully managed directories get a full recursive diff. Partially managed directories (like `~/`) only diff managed files — unmanaged siblings are ignored.

`dotd status env` checks that builtins are detected, context is set, the rc file is wired, and the repo is not behind origin.

---

## 11. CLI Interface

```
dotd <command> [options]
```

### Core commands

| Command | Description |
|---------|-------------|
| `dotd install` | Set up dot-dagger — rc wiring, first-run env prompts |
| `dotd install --apply` | Set up then immediately apply |
| `dotd apply` | Full reconciliation — evaluate predicates, resolve DAG, symlink, generate `init.sh` |
| `dotd diff` | Show what apply would change |
| `dotd check` | Validate predicates, DAG, annotations, `.dotd.yaml` files |
| `dotd status` | Full status report |
| `dotd status config` | Config and annotation validation |
| `dotd status env` | Environment health |
| `dotd status files` | Filesystem drift |
| `dotd doctor` | Analyse status, propose and optionally apply fixes |
| `dotd add <file>` | Begin tracking a file — fuzzy picker or `--module` |
| `dotd uninstall <path>` | Remove artifacts for files under a path |
| `dotd uninstall --all` | Remove everything |

### `dotd module` subcommands

| Command | Description |
|---------|-------------|
| `dotd module create <n>` | Scaffold a new directory with `scripts/`, `bin/`, `dots/`, and optional `.dotd.yaml` |
| `dotd module list` | List all directories with their active file counts |

### `dotd env` subcommands

| Command | Description |
|---------|-------------|
| `dotd env list` | Show all env key-value pairs and their sources |
| `dotd env get <key>` | Get a specific key |
| `dotd env set <key=val>` | Set a key in `env.yaml` |

### Global flags

| Flag | Description |
|------|-------------|
| `--force` | Override safety checks |
| `--dry-run` | Print actions without executing. Does not invoke annotation handlers. |
| `--env <key=val>` | Override env key for this invocation |
| `--interactive` / `--no-interactive` | Force interactivity mode |
| `--verbose` | Detailed output |
| `--all` | Required for unscoped destructive operations |

Interactivity defaults to `--interactive` when a TTY is detected, `--no-interactive` otherwise.

---

## 12. Output Style

Colored emoji output with plain ASCII fallback when the terminal does not support it.

```
✂  dot-dagger
✅  Environment resolved  os=macos  shell=zsh  context=personal
✅  12 nodes active
✅  8 symlinks applied
✅  init.sh generated  ~/.config/dot-dagger/init.sh
✅  Source line already present in ~/.zshrc
✨  Done!
```

| Symbol | Fallback | Meaning |
|--------|----------|---------|
| ✅ | `[ok]` | Success |
| ❌ | `[err]` | Error — halts |
| ⚠️ | `[warn]` | Warning — continues |
| ❓ | `[?]` | Prompt |
| ⏭ | `[skip]` | Skipped |
| 🔄 | `[...]` | In progress |

---

## 13. Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/<user>/dot-dagger/main/bootstrap.sh | sh
```

`bootstrap.sh` downloads the appropriate pre-built binary, prompts for the dotfiles repo URL, clones the repo, and runs `dotd install --apply`.

---

## 14. Go Project Structure

```
dot-dagger/
├── cmd/
│   └── dotd/
│       └── main.go
├── internal/
│   ├── predicate/      # predicate parser and evaluator
│   │   ├── ast.go      # Expr interface, node types, And(), Keys()
│   │   ├── lexer.go    # tokenizer
│   │   ├── parser.go   # recursive descent, Parse()
│   │   └── eval.go     # Env, Evaluator (injectable LookPath), Eval()
│   ├── annotation/     # annotation scanner
│   │   └── annotation.go  # Annotations, Custom, Scan(io.Reader)
│   ├── dotdyaml/       # .dotd.yaml loader and validator
│   │   └── dotdyaml.go    # DotD, Load(io.Reader), LoadFile(path)
│   ├── env/            # environment resolution
│   │   ├── env.go      # Schema, Resolver, MissingKeysError, Resolve()
│   │   └── detect.go   # builtin detectors for os, distro, shell
│   ├── graph/          # DAG builder and resolver
│   │   ├── node.go     # Node, NodeKind, LogicalNameFor, KindFor
│   │   ├── walk.go     # directory traversal, annotation merging
│   │   └── graph.go    # Build(), conflict detection, topo sort
│   ├── linker/         # symlink management
│   │   └── linker.go   # DestFor, Linker, Apply, Remove, Status
│   ├── initgen/        # init.sh generator
│   │   └── initgen.go  # Generator, Generate, WriteFile (atomic)
│   ├── state/          # drift detection (planned)
│   └── cli/            # cobra command implementations (planned)
├── go.mod
├── go.sum
├── SPEC.md
└── README.md
```

---

## 15. Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/stretchr/testify` | Test assertions |

---

## 16. Design Decisions and Rationale

| Decision | Rationale |
|----------|-----------|
| Go, single binary | No runtime dependency. Distributable via curl. Fast startup. |
| Convention over config | `scripts/`, `bin/`, `dots/` directories just work. `.dotd.yaml` only when needed. |
| Full dot-path logical names | Always predictable. No skipping of unnamed directories. `@name` for aliasing when paths are too long. |
| `@name` replaces full logical name | Enables variant files cleanly. Two files share a `@name`, predicates must be mutually exclusive. |
| `@after` is ordering only | Never affects inclusion. Missing or inactive targets are no-ops, never errors. |
| Alphabetical default ordering | Deterministic `init.sh` without requiring explicit `@after` on every file. Kahn's algorithm with alphabetical tie-breaking at each frontier step gives a fully deterministic total order. |
| Directory `when` as shared default | Avoids repeating the same predicate on every non-annotatable file in a directory. Cascades to all subdirectories, not just immediate files. |
| `.dotd.yaml` purpose is fallback | Primary use is metadata for files that cannot carry annotations. Not a module manifest. |
| Missing required keys prompt or halt | Never silently exclude files due to unset keys. Always surface the issue. |
| `nosync-` gitignore enforced at install | Prevents accidental staging of private files before the user has set up gitignore. |
| Symlink destination conflict detection | Separate from logical name conflicts. Two `@symlink` to same path with overlapping predicates is an error. |
| Custom annotation dispatch | Extends dot-dagger without modifying it. External tools own their annotations. |
| `--dry-run` skips annotation handlers | Annotation handlers have side effects. dry-run must be fully safe. |
| `exists()` predicate function | Capability gating without package manager coupling. |
| Two distinct `dot-` transformations | Logical names strip `dot-` entirely (DAG identity). Symlink destinations replace `dot-` with `.` (filesystem convention). Different rules, different functions. |
| `dot-` replacement applies at every level | Consistent rule — any path component starting with `dot-` gets its prefix replaced with `.`. `@retain-prefix` opts out per file's last component. |
| `directory` and `defaults` sections in `.dotd.yaml` | Clear separation between properties of the directory node itself and defaults that cascade to contents. |
| `files.path` uses true filename, predicates use logical names | Consistent with how annotations work — the filesystem is addressed by real name, the DAG by logical name. |
| Single shell-agnostic `init.sh` | One source line in any rc file. Shell-specific content handled by predicates. Uses POSIX `.` not bash `source`. |
| Single-quote shell paths with `'\''` | Universally safe quoting for sh/bash/zsh. `${HOME}` prefix for portability across machines. |
| Atomic `init.sh` write | Temp file + rename. A crash during apply never leaves a partial init.sh. |
| Symlink ownership by repo root prefix | Symlinks pointing into the repo are owned and updated freely. Foreign symlinks require `--force`. |
| No state file | Runtime scan from active nodes. Never out of sync. |
| `Resolve()` never prompts | Returns `*MissingKeysError`; the CLI catches with `errors.As` and decides whether to prompt or halt based on TTY. |
| Injectable test seams everywhere | `Evaluator.LookPath`, `Resolver.Detectors`, `Resolver.RunCmd`, `walker.readAnnotations`, `walker.readDotdYaml` — all injectable. Real implementations are zero values or defaults. |
| All errors collected before returning | `dotd check` shows every problem at once, not just the first one. `errors.Join` throughout. |
| Separate package procurement tool | dot-dagger stays focused. Package management is a well-defined standalone problem. |
| Modular internal packages | Each internal package independently testable. Clean boundaries. |

---

## 17. Out of Scope for v1

Encryption of secrets, Windows support, GUI or TUI, git sync commands, directory merging, `on_deactivate` config option, OS alias strict mode, shell-type predicates (interactive or login shell detection), and package manager integration (belongs to the separate tool).

---

## 18. Implementation Status

| Package | Status | Tests |
|---------|--------|-------|
| `internal/predicate` | 🔲 Planned | — |
| `internal/annotation` | 🔲 Planned | — |
| `internal/dotdyaml` | 🔲 Planned | — |
| `internal/env` | 🔲 Planned | — |
| `internal/graph` | 🔲 Planned | — |
| `internal/linker` | 🔲 Planned | — |
| `internal/initgen` | 🔲 Planned | — |
| `internal/state` | 🔲 Planned | — |
| `internal/cli` | 🔲 Planned | — |
| `cmd/dotd` | 🔲 Planned | — |

---

## 19. Open Questions

None — all design decisions resolved.
