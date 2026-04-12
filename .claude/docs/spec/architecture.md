# §14–§19 — Architecture, Dependencies, Design Decisions & Status

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

Nothing is implemented yet. All packages are planned.

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
