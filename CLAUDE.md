# CLAUDE.md

This file contains guidance for Claude when working in this repository.

This is a living document. As we discuss conventions, preferences, and project decisions, relevant guidance should be added here. For example, if I ask you to write tests for a module, infer that running tests is part of validation going forward.

## Branching Strategy

This project uses trunk-based development:

- `main` — default branch, always stable
- `feature/<name>` — all feature work branches off `main` and PRs back into it
- Releases are triggered by pushing a semver tag (e.g. `v0.1.0`) to `main`

### Branch Naming

- Human-authored branches: `feature/<name>`
- Claude-authored branches: `feature/claude-<name>`

The `feature/claude-` prefix makes it visually clear the branch was Claude's work.

## Repository

This is the `dot-dagger` repository — a home for Dagger pipelines and CI/CD configuration.

## Commit and Push Cadence

Commit and push fairly often. Before committing, validate that things are in a good state.

### Validation steps

- _(More steps will be added as the project grows — e.g. running tests once they exist)_

## Documentation

Claude reference docs live in `.claude/docs/`. These are works-in-progress intended as context for Claude, not general project documentation.

## TODO / Deferred Tasks

`.claude/TODO.md` tracks known deferred items. Keep it up to date as tasks are completed or new ones come up.
