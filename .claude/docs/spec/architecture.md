# §14–§19 — Architecture, Dependencies, Design Decisions & Status

## 14. Go Project Structure

```
dot-dagger/
├── cmd/
│   └── dotd/
│       ├── main.go       # root command, global flags, apply/check/completion
│       ├── link.go       # link apply/check/remove
│       ├── dag.go        # dag apply/check
│       ├── env.go        # env show/get/set/diff
│       ├── package.go    # package check/list/generate
│       ├── setup.go      # setup
│       └── adopt.go      # adopt
├── internal/
│   ├── annotation/     # annotation scanner
│   ├── dag/            # DAG builder, topo sort, conflict detection
│   ├── daggeryaml/     # .dotd.yaml loader and validator
│   ├── ecosystem/      # shared tool name constants
│   ├── env/            # environment resolution and auto-detection
│   ├── fileset/        # active file set construction from walk + env
│   ├── initgen/        # init.sh generator (atomic write)
│   ├── linker/         # symlink management — apply, remove, status
│   ├── packages/       # package catalog and install script generation
│   ├── predicate/      # predicate parser and evaluator
│   ├── setup/          # interactive onboarding logic
│   ├── ui/             # output formatting and colored cobra help
│   └── walk/           # directory traversal, annotation merging
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

---

## 16. Design Decisions and Rationale

| Decision | Rationale |
|----------|-----------|
| Go, single binary | No runtime dependency. Distributable via curl. Fast startup. |
| Convention over config | `shellrc/`, `bin/`, `conf/` directories just work. `.dotd.yaml` only when needed. Convention names overridable via `dotd.conventions` in root `.dotd.yaml`. |
| Full dot-path logical names | Always predictable. No skipping of unnamed directories. `@name` for aliasing when paths are too long. |
| `@name` replaces full logical name | Enables variant files cleanly. Two files share a `@name`, predicates must be mutually exclusive. |
| `@after` is ordering only | Never affects inclusion. Missing or inactive targets are no-ops, never errors. |
| Alphabetical default ordering | Deterministic `init.sh` without requiring explicit `@after` on every file. Kahn's algorithm with alphabetical tie-breaking at each frontier step gives a fully deterministic total order. |
| Directory `when` as shared default | Avoids repeating the same predicate on every non-annotatable file in a directory. Cascades to all subdirectories, not just immediate files. |
| `.dotd.yaml` purpose is fallback | Primary use is metadata for files that cannot carry annotations. Not a module manifest. |
| Missing required keys prompt or halt | Never silently exclude files due to unset keys. Always surface the issue. |
| `nosync-` is user responsibility | `nosync-` is stripped at runtime. User must gitignore. `dotd setup` and `dotd check` warn and offer to add `nosync-*` to `.gitignore` if missing — never silently. |
| Symlink destination conflict detection | Separate from logical name conflicts. Two `@symlink` to same path with overlapping predicates is an error. |
| Custom annotation dispatch | Extends dot-dagger without modifying it. External tools own their annotations. |
| `--dry-run` skips annotation handlers | Annotation handlers have side effects. dry-run must be fully safe. |
| `exists()` predicate function | Capability gating without package manager coupling. |
| Two distinct `dot-` transformations | Logical names strip `dot-` entirely (DAG identity). Symlink destinations replace `dot-` with `.` (filesystem convention). Different rules, different functions. |
| `dot-` transformation applies uniformly | Any path component starting with `dot-` gets its prefix replaced with `.`. Files and directories follow the same rule. `@retain-prefix` opts out for a specific component. |
| `conf/` renamed from `dots/` | `conf/` more precisely describes the purpose: config files that third-party tools expect at a fixed path. `dots/` implied "dotfiles broadly" which was misleading. |
| `conf/` symlinks relative to `~` by default | `link_root` in `.dotd.yaml` allows overriding the base path per subtree. Cascades to subdirectories. |
| `@symlink` path is implicit-relative | Absolute if starting with `/` or `~/`, otherwise relative to `link_root`. No new sigil needed — mirrors Unix path conventions. |
| Convention dirs recognised until first encounter | Once inside a convention dir, further convention dirs inside it are ignored. Prevents confusing nesting without a hard depth cap. Allows `nosync-work/tmux/shellrc/` and other deep but legitimate layouts. |
| `dotd` and `link` sections in `.dotd.yaml` | `dotd` owns directory/file metadata (when, defaults, files list). `link` owns symlink config (link_root). Clear separation of concerns. |
| `files.path` uses true filename, predicates use logical names | Consistent with how annotations work — the filesystem is addressed by real name, the DAG by logical name. |
| Single shell-agnostic `init.sh` | One source line in any rc file. Shell-specific content handled by predicates. Uses POSIX `.` not bash `source`. |
| Single-quote shell paths with `'\''` | Universally safe quoting for sh/bash/zsh. `${HOME}` prefix for portability across machines. |
| Atomic `init.sh` write | Temp file + rename. A crash during apply never leaves a partial init.sh. |
| Symlink ownership by repo root prefix | Symlinks pointing into the repo are owned and updated freely. Foreign symlinks require `--force`. |
| No state file | Runtime scan from active nodes. Never out of sync. |
| Single `dotd check` command | Replaces separate `dotd status` and `dotd check`. One command covers state inspection and error detection. Expandable to subcommands later if needed. |
| No `dotd diff` command | Deployment artifacts are symlinks and `init.sh`. `dotd apply --dry-run` covers the preview use case. A dedicated diff command adds complexity without meaningful value. |
| No modules concept | Directories are the natural organisational unit. `@module` and `dotd module` subcommands removed. |
| `Resolve()` never prompts | Returns `*MissingKeysError`; the CLI catches with `errors.As` and decides whether to prompt or halt based on TTY. |
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
| `internal/daggeryaml` | ✅ Implemented |
| `internal/env` | ✅ Implemented |
| `internal/dag` | ✅ Implemented |
| `internal/walk` | ✅ Implemented |
| `internal/fileset` | ✅ Implemented |
| `internal/linker` | ✅ Implemented |
| `internal/initgen` | ✅ Implemented |
| `internal/packages` | ✅ Implemented |
| `internal/manifest` | ✅ Implemented — parses `dotd-packages.yaml` files, evaluates block predicates |
| `internal/composer` | ✅ Implemented — compose target detection, fragment ordering, file generation |
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
| `dotd-packages.yaml` / `*.dotd-packages.yaml` naming | Avoids collision with user-owned `*.packages.yaml` files used by other tools. |
| Package manifests excluded from DAG | They declare desired state, not shell behaviour. No ordering, no sourcing, no logical name. |
| Block-level `when`, no file-level `when` | File-level predicate is handled by directory `.dotd.yaml` `when` — same mechanism as all other files. No new concept needed. |
| Manifests contribute to same package catalog as `@request` | Single unified source for `dotd package` commands regardless of declaration location. |
| Compose targets require explicit `compose: true` | No implicit detection. Prevents accidental misclassification. Enables validation of fragment annotations. |
| Compose generates to managed dir, downstream stages unaware | Generated file is a synthetic node of the appropriate kind (`KindScript`, `KindConf`, `KindBin`). Linker, init generator, drift detection all work identically. |
| Compose works in any convention dir | Not conf/-specific. Output kind determined by parent context — shellrc → sourced, conf → symlinked, bin → symlinked + executable. |
| `dotd.name` overrides compose target logical name | Consistent with `@name` for annotatable files. Directory-level `.dotd.yaml` is the established mechanism for metadata on non-annotatable nodes. |
| Compose pipeline stage between fileset and links | Generated files must exist before the linker runs. Compose produces synthetic `KindConf` nodes for the linker. |
