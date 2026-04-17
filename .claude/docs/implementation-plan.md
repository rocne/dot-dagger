# Implementation Plan

**Status: complete.** All phases shipped as single `dotd` binary.

## What was built

| Package | Purpose |
|---------|---------|
| `internal/annotation` | File comment scanner; extracts `@key value` pairs |
| `internal/predicate` | AST, lexer, parser, evaluator; `Env map[string]string` input |
| `internal/daggeryaml` | Loads `.dotd.yaml`; sections: `dotd`, `link`, `env` |
| `internal/env` | OS/distro/shell detectors; loads `env.yaml`; `MissingKeysError` |
| `internal/walk` | Traverses dotfiles tree; attaches annotations + config data |
| `internal/fileset` | Filters walked tree by predicate evaluation |
| `internal/dag` | DAG from `@after`/`@name`; Kahn's + alpha tie-break |
| `internal/initgen` | Renders `init.sh` from DAG; atomic write |
| `internal/linker` | Symlink apply/check/remove; drift detection |
| `internal/packages` | `packages.yaml` registry; `@require`/`@request`; script generation |
| `cmd/dotd` | Single binary: env, dag, link, package, setup, adopt subcommands |
