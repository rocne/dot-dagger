# §14–§19 — Architecture, Dependencies, Design Decisions & Status

## 14. Go Project Structure

```
dot-dagger/
├── cmd/
│   └── dotd/
│       ├── main.go         # root command, global flags, apply/check/completion
│       ├── adopt.go        # dotd adopt
│       ├── bundle.go       # dotd bundle
│       ├── compose_cmd.go  # dotd compose check/list
│       ├── config_cmd.go   # dotd config show/get/set/edit
│       ├── dag_cmd.go      # dotd dag check
│       ├── env.go          # dotd env show/get/set/diff/edit
│       ├── getters.go      # hidden: dotd get-os, dotd get-hostname
│       ├── init_cmd.go     # dotd init
│       ├── list_cmd.go     # dotd list
│       └── package.go      # dotd package list/check/generate
├── internal/
│   ├── adopter/    # adopt logic — copy, remove, symlink
│   ├── annotation/ # annotation scanner (@key, @key(args))
│   ├── config/     # config.yaml loader
│   ├── dagger/     # .dagger file loader and types
│   ├── ecosystem/  # shared constants (file names, dirs)
│   ├── env/        # env.yaml loader and key resolution
│   ├── manifest/   # dotd-packages.yaml parser (unused — to be removed)
│   ├── node/       # logical name derivation
│   ├── packages/   # package registry, catalog, install script generation
│   ├── pipeline/   # walk → filter → order → act → initgen
│   ├── predicate/  # predicate parser and evaluator
│   ├── setup/      # dotd init scaffolding, source-line helpers
│   └── ui/         # output formatting and coloured cobra help
├── go.mod
├── go.sum
└── README.md
```

---

## 15. Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/charmbracelet/huh` | Interactive prompts (adopt confirm) |
| `github.com/charmbracelet/log` | Structured logging |
| `github.com/charmbracelet/x/term` | TTY detection |

---

## 16. Design Decisions and Rationale

| Decision | Rationale |
|----------|-----------|
| Go, single binary | No runtime dependency. Distributable via curl. Fast startup. |
| Convention over config | `shellrc/`, `bin/`, `conf/` directories just work. `.dagger` only when needed. Convention names overridable via `dotd.conventions` in root `.dagger`. |
| Full dot-path logical names | Always predictable. No skipping of unnamed directories. `@name` for aliasing when paths are too long. |
| `@name` replaces full logical name | Enables variant files cleanly. Two files share a `@name`, predicates must be mutually exclusive. |
| `@after` is ordering only | Never affects inclusion. Missing or inactive targets are no-ops, never errors. |
| Alphabetical default ordering | Deterministic `init.sh` without requiring explicit `@after` on every file. Kahn's algorithm with alphabetical tie-breaking at each frontier step gives a fully deterministic total order. |
| Directory `when` as shared default | Avoids repeating the same predicate on every non-annotatable file in a directory. Cascades to all subdirectories, not just immediate files. |
| `.dagger` purpose is fallback | Primary use is metadata for files that cannot carry annotations. Not a module manifest. |
| Missing required keys prompt or halt | Never silently exclude files due to unset keys. Always surface the issue. |
| `nosync-` is user responsibility | `nosync-` is stripped at runtime. User must gitignore. `dotd setup` and `dotd check` warn and offer to add `nosync-*` to `.gitignore` if missing — never silently. |
| Symlink destination conflict detection | Separate from logical name conflicts. Two `@symlink` to same path with overlapping predicates is an error. |
| Custom annotation dispatch | Extends dot-dagger without modifying it. External tools own their annotations. |
| `--dry-run` skips annotation handlers | Annotation handlers have side effects. dry-run must be fully safe. |
| `exists()` predicate function | Capability gating without package manager coupling. |
| Two distinct `dot-` transformations | Logical names strip `dot-` entirely (DAG identity). Symlink destinations replace `dot-` with `.` (filesystem convention). Different rules, different functions. |
| `dot-` transformation applies uniformly | Any path component starting with `dot-` gets its prefix replaced with `.`. Files and directories follow the same rule. |
| `conf/` renamed from `dots/` | `conf/` more precisely describes the purpose: config files that third-party tools expect at a fixed path. `dots/` implied "dotfiles broadly" which was misleading. |
| `conf/` symlinks relative to `~` by default | `link_root` in `.dagger` allows overriding the base path per subtree. Cascades to subdirectories. |
| `@symlink` path is implicit-relative | Absolute if starting with `/` or `~/`, otherwise relative to `link_root`. No new sigil needed — mirrors Unix path conventions. |
| Convention dirs are naming + prepopulated `.dagger` defaults | `shellrc/`, `bin/`, `conf/` are not special at the system level. Their behavior comes from `defaults.actions` in the dir's `.dagger` file. The convention is naming + prepopulated config, not implicit magic. |
| `.dagger` is flat — no section nesting | All fields (`when`, `link_root`, `actions`, `defaults`, `files`, `composition`, `conventions`, `name`) are top-level. No `dotd:` or `link:` wrapping. |
| `files` map uses true filename as key | Consistent with how annotations work — the filesystem is addressed by real name, the DAG by logical name. |
| Single shell-agnostic `init.sh` | One source line in any rc file. Shell-specific content handled by predicates. Uses POSIX `.` not bash `source`. |
| Single-quote shell paths with `'\''` | Universally safe quoting for sh/bash/zsh. `${HOME}` prefix for portability across machines. |
| Atomic `init.sh` write | Temp file + rename. A crash during apply never leaves a partial init.sh. |
| Symlink ownership by repo root prefix | Symlinks pointing into the repo are owned and updated freely. Foreign symlinks require `--force`. |
| No state file | Runtime scan from active nodes. Never out of sync. |
| Single `dotd check` command | Replaces separate `dotd status` and `dotd check`. One command covers state inspection and error detection. Expandable to subcommands later if needed. |
| No `dotd diff` command | Deployment artifacts are symlinks and `init.sh`. `dotd apply --dry-run` covers the preview use case. A dedicated diff command adds complexity without meaningful value. |
| No modules concept | Directories are the natural organisational unit. `@module` and `dotd module` subcommands removed. |
| Missing keys always halt with a hint | `Resolve()` returns `*MissingKeysError`; the CLI annotates it with `"Hint: set it with --env ..."` and exits. |
| Injectable test seams everywhere | `Evaluator.LookPath`, `Resolver.Detectors`, `walker.readAnnotations`, `walker.readDotdYaml` — all injectable. Real implementations are zero values or defaults. |
| All errors collected before returning | `dotd check` shows every problem at once, not just the first one. `errors.Join` throughout. |
| Separate package procurement tool | dot-dagger stays focused. Package management is a well-defined standalone problem. |
| Modular internal packages | Each internal package independently testable. Clean boundaries. |

---

## 17. Out of Scope for v1

Encryption of secrets, Windows support, GUI or TUI, git sync commands, directory merging, `on_deactivate` config option, OS alias strict mode, shell-type predicates (interactive or login shell detection), and package manager integration (belongs to the separate tool).

---

## 18. Implementation Status

All packages implemented and tested.

| Package | Status |
|---------|--------|
| `internal/predicate` | ✅ Implemented |
| `internal/annotation` | ✅ Implemented |
| `internal/dagger` | ✅ Implemented — `.dagger` loader and types |
| `internal/env` | ✅ Implemented |
| `internal/config` | ✅ Implemented |
| `internal/node` | ✅ Implemented — logical name derivation |
| `internal/pipeline` | ✅ Implemented — walk, filter, order, act, initgen |
| `internal/packages` | ✅ Implemented |
| `internal/manifest` | ⚠️ Implemented but unused — to be removed |
| `internal/adopter` | ✅ Implemented |
| `internal/setup` | ✅ Implemented |
| `internal/ecosystem` | ✅ Implemented |
| `internal/ui` | ✅ Implemented |
| `cmd/dotd` | ✅ Implemented |

---

## 19. Open Questions

None — all design decisions resolved.

---

## Additional Design Decisions

| Decision | Rationale |
|----------|-----------|
| `.dagger` files: dict supports all annotations | Every annotation (`@when`, `@after`, `@name`, `@action`, `@require`, `@request`, `@disable`) can be expressed in `.dagger` for files that cannot carry comment annotations. No separate manifest format needed. |
| Compose targets require explicit `compose: true` | No implicit detection. Prevents accidental misclassification. Enables validation of fragment annotations. |
| Compose generates to managed dir | Generated file written to `~/.local/share/dot-dagger/generated/`. Downstream stages (linker, init generator) consume it like any other file. |
| Compose works anywhere | Not tied to convention dirs. Output behavior declared via `actions:` on the compose target — `source`, `link(dest)`, or both. |
| `name:` overrides compose target logical name | Consistent with `@name` for annotatable files. Directory-level `.dagger` is the mechanism for metadata on non-annotatable nodes. |
| Compose pipeline stage between fileset and links | Generated files must exist before the linker runs. |
