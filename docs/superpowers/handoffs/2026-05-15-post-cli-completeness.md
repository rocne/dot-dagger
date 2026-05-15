# Session Handoff — 2026-05-15

## What just merged

**PR #62 — CLI Completeness + Refactoring** (`feature/claude-cli-completeness` → `main`, merge commit `9027b4b`)

### Changes

- **`runPipeline` refactor** (`cmd/dotd/main.go`) — extracted shared walk→validate→filter→order→act setup from `runApply` and `runCheck` into a `runPipeline(cfg, dryRun)` helper + `pipelineRun` struct. Preserves all logging including `resolvedCount` for the env debug line.

- **`dotd env diff`** (`cmd/dotd/env.go`) — new subcommand. Compares expanded `env.yaml` values against `DOTD_*` shell vars; shows what the file contributes. Intentionally does NOT include `--env` CLI overrides (compares file vs shell, not final resolved env — documented in a comment). 3 tests in `main_test.go`.

- **`dotd package list`** (`cmd/dotd/package.go`) — new subcommand. Lists `@require`/`@request` packages from active nodes, deduplicated, without checking install status. 3 tests in `main_test.go`.

- **Unhide `package`, `compose`, `dag`** — removed `Hidden: true` from all three command groups. They're now visible in `dotd --help`.

- **`cli.md` spec update** (`.claude/docs/spec/cli.md`) — replaced stale v1 tables (`dotd link`, `dotd files`, `dotd setup`) with accurate v2 tables. Added `dotd list`, `dotd env diff`, `dotd config` sections. Descriptions match actual `Short` fields in code.

- **`internal/setup` tests** (`internal/setup/setup_test.go`) — 5 tests for `Scaffold`: creates dirs, creates files, idempotency, selected managers, detected values.

- **`internal/ecosystem` tests** (`internal/ecosystem/ecosystem_test.go`) — 8 tests for `ResolvePath` precedence (cli > shell > file > default) and `Default*` XDG path functions.

---

## State of TODO.md

`.claude/TODO.md` does not need updating — nothing from the deferred list was touched. All items remain as-is.

---

## Open deferred items (unchanged)

| Item | Status |
|------|--------|
| `CONTRIBUTING.md` | Blocked on going public |
| Go public (GitHub visibility) | Owner decision |
| GitHub Pages | Blocked on going public |
| Convention dir defaults (`shellrc/`, `conf/`, `bin/`) | P3, no urgency |
| Multi-distro Docker tests | Blocked on going public |
| `dotd adopt` migration | Stubbed (`"not yet migrated to v2 pipeline"`), no plan yet |

---

## Remaining CLI gaps

`dotd adopt` is the only stubbed command — returns an error if invoked. When ready to tackle it, it needs:
- Reading the old adopt logic (pre-v2) from git history to understand what it did
- Implementing against the v2 pipeline (Walk result, symlink creation via `pipeline.Act`)
- A separate plan — it's a non-trivial feature

---

## Branch state

`feature/claude-cli-completeness` merged and can be deleted locally:

```sh
git branch -d feature/claude-cli-completeness
```

---

## Spec state

`.claude/docs/spec/cli.md` is current and accurate as of this merge. All other spec files unchanged.
