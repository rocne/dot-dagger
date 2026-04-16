# Implementation Plan

All work happens in the current repo (`dot-dagger`), renamed to `dotr` when convenient. Module path is `github.com/rocne/dotr` from day one.

---

## Phase 0 — Repo Bootstrap

Get the repo into a state where Go code can be written and CI will run.

- [ ] Add `LICENSE` (MIT)
- [ ] `go.mod` — module `github.com/rocne/dotr`, current stable Go version
- [ ] Create directory skeleton: `internal/`, `cmd/`
- [ ] `.github/workflows/ci.yml` — `go build ./...` + `go test ./...`
- [ ] `.github/workflows/lint.yml` — `golangci-lint`
- [ ] `.github/pull_request_template.md`

**Done when:** CI passes on an empty commit, lint runs clean.

---

## Phase 1 — Foundation

The two packages with no internal dependencies. Everything else builds on these.

- [ ] `internal/annotation` — scanner; reads file comments and extracts `@key value` pairs
- [ ] `internal/predicate` — AST, lexer, parser, evaluator; takes `Env map[string]string` and a predicate function registry as input

**Done when:** both packages have full unit test coverage; predicate evaluator handles all grammar cases from v1 spec.

---

## Phase 2 — Config + Env

Loads configuration and resolves the runtime environment.

- [ ] `internal/daggeryaml` — load and validate `.dot-dagger.yaml`; typed structs per tool section
- [ ] `internal/env` — built-in detectors (OS, distro, shell); loads `env.yaml`; produces `Env` map; `MissingKeysError`
- [ ] `cmd/dote` — `dote show`; first working CLI; validates env pipeline end-to-end

**Done when:** `dote show` prints a resolved env map for a real dotfiles directory.

---

## Phase 3 — Walk + FileSet

The shared context that all downstream stages consume. Must be solid before Phase 4.

- [ ] `internal/walk` — traverse dotfiles directory tree; attach annotations and `.dot-dagger.yaml` data to each node; handle special dirs (`scripts/`, `conf/`, `bin/`)
- [ ] `internal/fileset` — filter a walked tree by predicate evaluation; produce `fileset.Set` with active nodes partitioned by kind

**Done when:** given a real dotfiles directory, `fileset.Set` correctly contains only the nodes whose `@when` conditions pass; `@when`-less nodes always included.

---

## Phase 4 — Core Tool Logic

Can be developed in parallel once Phase 3 is complete.

### DAG + init.sh
- [ ] `internal/dag` — build DAG from `@after`/`@name` annotations; Kahn's algorithm with alphabetical tie-breaking; conflict detection
- [ ] `internal/initgen` — render `init.sh` from resolved DAG; atomic write

### Linker
- [ ] `internal/linker` — `DestFor` (resolve symlink destination); `Apply`, `Remove`, `Status`; drift detection; `@symlink`, `@retain-prefix` support

### Packages
- [ ] `internal/packages` — load `packages.yaml`; register `@require`/`@request` annotation handlers and `installed()`/`installable()` predicates; install/check/list logic

**Done when:** each package has unit tests covering its core logic; can be exercised against a real dotfiles directory via tests.

---

## Phase 5 — CLIs + Orchestrator

Thin wrappers over Phase 4 logic.

- [ ] `cmd/dotd` — walk + fileset + dag + initgen; `dotd generate`
- [ ] `cmd/dotl` — standalone walk (unconditional) + linker; `dotl apply`, `dotl check`, `dotl remove`
- [ ] `cmd/dotp` — walk + fileset + packages; `dotp install`, `dotp check`, `dotp list`
- [ ] `cmd/dotr` — full orchestration; builds one FileSet, fans out to all stages

**Done when:** `dotr apply` on a real dotfiles directory produces correct `init.sh` and symlinks; composability parity validated — running `dotd` + `dotl` + `dotp` individually produces identical results.

---

## Validation strategy

Each phase builds on the previous. Validation at each phase boundary:

| Phase | Validation |
|-------|-----------|
| 0 | CI green on empty repo |
| 1 | Unit tests for annotation + predicate |
| 2 | `dote show` works on a real dotfiles dir |
| 3 | FileSet correctly filters a real dotfiles dir |
| 4 | Unit tests; package-level integration tests |
| 5 | End-to-end: `dotr` == individual tools composed |

---

## Deferred

- `internal/state` — drift detection across full active set; deferred until core apply/check cycle is working
- `link_root` / `@symlink` relative path edge cases — implement basic case first, revisit with real use cases
