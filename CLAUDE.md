# CLAUDE.md

This file contains guidance for Claude when working in this repository.

This is a living document. As we discuss conventions, preferences, and project decisions, relevant guidance should be added here. For example, if I ask you to write tests for a module, infer that running tests is part of validation going forward.

## Branching Strategy

This project uses trunk-based development:

- `main` — default branch, always stable
- `feature/<name>` — all feature work branches off `main` and PRs back into it

### Branch Naming

- Human-authored branches: `feature/<name>`
- Claude-authored branches: `feature/claude-<name>`

The `feature/claude-` prefix makes it visually clear the branch was Claude's work.

## Release Process

Two release paths exist:

- **Auto release** (`.github/workflows/auto-release.yml`) — triggers on every merge to `main` when `internal/**` or `cmd/dotd/**` changes. Auto-bumps the patch version. No manual action needed.
- **Manual release** (`.github/workflows/release.yml`) — push a tag to trigger a controlled release.

### Tag format

```
v<semver>
```

Example: `v0.2.0`

### How to manually release

```sh
git tag v0.2.0
git push origin v0.2.0
```

This triggers `.github/workflows/release.yml`, which:
1. Runs GoReleaser to build linux+darwin × amd64+arm64 archives (`--skip=validate,publish`)
2. Creates the GitHub release via `gh release create` attached to the tag

### Re-triggering a release

If a release fails mid-run, delete and re-push the tag from `main`:

```sh
git push origin --delete v0.2.0
git tag -d v0.2.0
git tag v0.2.0   # ensure main is checked out
git push origin v0.2.0
```

Always tag from `main`. The workflow files must be present at the tagged commit — tagging a commit before the release workflow was merged will silently not trigger.

## Repository

This is the `dot-dagger` repository — a home for Dagger pipelines and CI/CD configuration.

## Commit and Push Cadence

Commit and push fairly often. Before committing, validate that things are in a good state.

**All changes go to a feature branch and merge via PR — never commit directly to `main`.** Conceptually and temporally related changes belong in the same branch and PR. Batch related work rather than opening many small PRs.

When a PR already exists for the current branch, update it rather than opening a new one. **Before every push, run `gh pr view` to confirm the PR is still open.** If it is merged, create a new branch and PR — never push to a merged branch.

### Validation steps

- _(More steps will be added as the project grows — e.g. running tests once they exist)_

## Documentation

Claude reference docs live in `.claude/docs/`. These are works-in-progress intended as context for Claude, not general project documentation.

## TODO / Deferred Tasks

`.claude/TODO.md` tracks known deferred items. Keep it up to date as tasks are completed or new ones come up.

## Token Efficiency

Follow these rules to avoid consuming unnecessary context.

### Session startup

At the start of each session, read:
- `CLAUDE.md` (this file)
- `.claude/TODO.md`

Defer all other files until the task requires them.

### Spec documents

The spec is split into focused sections under `.claude/docs/spec/`. Always start with `index.md` to identify which section is relevant, then read only that section. Never read the full spec speculatively.

| Task involves... | Read... |
|-----------------|---------|
| Predicates / conditions | `predicates.md` |
| DAG / ordering / annotations | `dag.md` |
| Symlinks | `symlinks.md` |
| Shell init generation | `shell-init.md` |
| CLI commands | `cli.md` |
| Config files | `env.md` |
| Architecture / structure | `architecture.md` |

### File reading discipline

- Use Grep to locate relevant code before reading whole files
- Read files incrementally (with offset/limit) when only part is needed
- Don't re-read a file to confirm something already established in the current conversation
