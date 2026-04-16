# Code Structure

## Repository and Module

Single repository (`dotr`), single Go module (`github.com/rocne/dotr`). No `go.work` needed.

```
dotr/
  internal/          ← all business logic
  cmd/               ← thin CLI entry points
  go.mod
  go.sum
```

`internal/` packages are not importable outside the module. This is intentional — dotr is a tool suite, not a library. The boundary can be relaxed later by moving packages out of `internal/` if external importability becomes valuable.

---

## Internal Packages

All business logic lives here. Named by concern, not by tool.

| Package | Responsibility |
|---------|---------------|
| `internal/annotation` | Scan file comments for `@key value` pairs; language-agnostic |
| `internal/predicate` | Parse and evaluate `@when` expressions; AST, lexer, parser, evaluator |
| `internal/daggeryaml` | Load and validate `.dot-dagger.yaml`; per-directory config |
| `internal/env` | Resolve environment: load `env.yaml`, run detectors, produce `Env` map |
| `internal/walk` | Traverse a dotfiles directory tree; read annotations and `.dot-dagger.yaml` per node |
| `internal/fileset` | The shared active-file context; filter a walked tree by predicate |
| `internal/dag` | Build a DAG from `@after`/`@name` annotations; topological sort |
| `internal/initgen` | Render and atomically write `init.sh` from a resolved DAG |
| `internal/linker` | Symlink apply, remove, check; destination resolution; drift detection |
| `internal/packages` | Package installation; registers `@package` handler and `installable()` predicate |
| `internal/state` | Drift detection across the full active set (planned) |

---

## Package Dependency Graph

```
annotation    predicate    dotryaml        (no internal deps)
     │              │          │
     └──────┬────── ┘          │
            │                  │
          walk ────────────────┘
            │
          env ── (dotryaml)
            │
         fileset ── (walk + predicate + env)
         /     \
       dag    linker    packages
        │
     initgen
```

`annotation` and `predicate` are the foundation — they have no internal dependencies. `fileset` is the central shared type that most other packages consume.

---

## The `FileSet`

`fileset.Set` is the shared in-memory context passed through the system. It is produced once (per run) and handed to each subsequent stage.

| Field | Type | Description |
|-------|------|-------------|
| `Nodes` | `[]Node` | Active files/dirs after predicate evaluation |
| `Env` | `map[string]string` | Fully resolved environment |

Each `Node` carries: filesystem path, logical name, kind (`script`, `conf`, `bin`, `other`), and resolved annotations.

Stages that operate on a `FileSet` never re-read the filesystem or re-evaluate predicates. They work on what's already been resolved.

---

## I/O Boundary

| Layer | Owns |
|-------|------|
| `internal/` | All logic; I/O primitives (file reads, atomic writes, symlink syscalls) |
| `cmd/` | Arg parsing, user prompts, stdout/stderr, exit codes, config path resolution |

The rule: if it touches `os.Args`, `fmt.Println`, or `os.Exit` — it belongs in `cmd/`. Everything else is `internal/`.

Example split for `dotd`:
- `internal/dag` — resolve ordering from a `FileSet`
- `internal/initgen` — render and write `init.sh` to a given path
- `cmd/dotd` — parse `--output` flag, resolve default path, call initgen, print result or error

---

## cmd/ Tools

Each binary is a thin shell. Its only jobs: parse input, call internal packages, handle output.

| Binary | Internal packages used | Standalone behaviour |
|--------|----------------------|----------------------|
| `cmd/dotd` | `walk`, `env`, `fileset`, `dag`, `initgen` | Walk `scripts/`, build FileSet, resolve DAG, write `init.sh` |
| `cmd/dotl` | `walk`, `linker` | Walk `conf/`, `bin/` unconditionally, apply symlinks |
| `cmd/dotp` | `walk`, `env`, `fileset`, `packages` | Walk all dirs, find `@package` annotations, install |
| `cmd/dote` | `env` | Resolve and print `Env` map |
| `cmd/dotr` | all | Build full FileSet, fan out to dag+initgen, linker, packages |

---

## Standalone vs Orchestrated

The same internal functions handle both modes. The difference is only in how the `FileSet` is built:

- **Standalone:** each `cmd/` tool builds its own `FileSet` scoped to its own directories
- **Orchestrated:** `cmd/dotr` builds one `FileSet` over the full dotfiles tree, then passes subsets to each stage

`linker.Apply(fs)`, `dag.Build(fs)`, `packages.Install(fs)` — identical call sites in both modes. No special orchestration paths inside `internal/`.

This directly supports the composability parity design goal: a user can replicate `dotr`'s behaviour by running each `cmd/` tool in sequence with the same dotfiles directory.
