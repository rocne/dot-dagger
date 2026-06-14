# Contributing to dotd

Thanks for your interest in contributing. This guide covers the branching model,
the Conventional-Commit PR-title contract (it drives releases), and how to
validate changes locally before opening a PR.

## Branching model

Trunk-based development. `main` is always stable and is the only long-lived
branch.

- Branch off `main`; PR back into `main`.
- Name feature branches `feature/<name>`.
- Never commit directly to `main` — all changes land via PR.

## PR titles are Conventional Commits (required)

PRs are **squash-merged**, so the **PR title becomes the commit subject on
`main`**. That subject is what [release-please](https://github.com/googleapis/release-please)
reads to compute the next version and build the changelog, so the title must
follow the [Conventional Commits](https://www.conventionalcommits.org/) format.
A CI check (`.github/workflows/pr-title.yml`) enforces this and will fail the PR
otherwise.

```
<type>[optional scope][!]: <description>
```

Examples:

```
fix: tolerate legacy config.yaml in path resolution
feat(dotd): report commit and build date in --version
docs: document the release-please flow
```

### How the type affects releases

While the project is pre-1.0:

| Type | Release effect |
|------|----------------|
| `fix:` | patch bump |
| `feat:` | minor bump |
| `feat!:` / `BREAKING CHANGE:` | minor bump (major is suppressed pre-1.0) |
| `docs:`, `chore:`, `ci:`, `style:`, `refactor:`, `test:`, `build:`, `perf:` | no release on their own |

A malformed or mis-typed title silently drops your change from the changelog and
release automation — that's why the check is a hard gate.

The commit *body* and the individual commits on your branch are not constrained;
only the PR title matters once squashed.

## Validating locally

CI runs lint, build, unit + integration tests, and an end-to-end suite on every
PR (`.github/workflows/ci.yml`, `lint.yml`). Run the fast checks before pushing:

```sh
gofmt -l .                              # should print nothing
golangci-lint run                       # lint (CI pins v2.11.4)
go build ./...                          # build
go test ./...                           # unit tests
go test -tags integration ./cmd/dotd/   # integration tests
```

The end-to-end suite builds `dotd` and exercises it inside Docker:

```sh
./test/run-e2e.sh                       # requires Docker
```

e2e is the canonical pre-merge gate and always runs in CI, so running it locally
is optional — do it if your change touches install, symlink, or shell-init
behavior and you have Docker available.

## Opening a PR

1. Branch off `main`.
2. Make your change; keep conceptually related work in one branch/PR rather than
   splitting into many tiny PRs.
3. Run the local checks above.
4. Open the PR with a Conventional-Commit title. Keep the PR scoped — the title
   should accurately describe the squashed change.
5. CI must be green (lint, tests, e2e, PR-title check) before merge.

## How releases happen

You don't tag or publish anything. release-please maintains a standing release PR
(`chore(main): release <version>`) that accumulates merged changes; a maintainer
merges it to cut the release. See the "Release Process" section of
[`CLAUDE.md`](CLAUDE.md) for the full mechanics.
