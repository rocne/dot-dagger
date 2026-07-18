# CLAUDE.md

This file contains guidance for Claude when working in this repository.

This is a living document. As we discuss conventions, preferences, and project decisions, relevant guidance should be added here. For example, if I ask you to write tests for a module, infer that running tests is part of validation going forward.

## Project Philosophy

This is a personal project, but it is **deliberately engineered to a higher standard than its scale demands.** Roughly half the goal is real distribution; the other half is a working exercise in robust, industry-standard release and distribution engineering.

That means the usual "is this overkill for a personal project?" instinct is **inverted here**: machinery that looks excessive for the audience size (release-please, Conventional-Commit-derived versioning, GPG/cosign signing, self-managed package repos, full CI release pipelines) is **intentional and in scope**, because exercising the robust real-world pattern is itself a goal.

Apply this when proposing or evaluating work:

- **Default toward the robust, real-world pattern** — the one a serious org or platform team would run — over the minimal solution that merely works, even when the minimal one would suffice for the current audience.
- **Prefer the path that teaches/exercises the mechanism** over a black-box shortcut, when both deliver. (E.g. self-managed GPG index signing over server-side magic; writing the release glue over an opaque convenience block.)
- **The guard against cargo-culting** is a two-part litmus, applied to every addition: *does it teach a real, transferable pattern, OR serve a real install/use?* If **neither**, it's out — "robust" is not a license for ornamentation with no learning or distribution payoff.
- This philosophy does **not** override effort that demands ongoing external commitment with no payoff here — e.g. official distro-repo (Fedora/EPEL, Debian/Ubuntu) maintainership is correctly out of scope: months of process, no transferable-in-a-sandbox lesson, no real install need.

## Branching Strategy

This project uses trunk-based development:

- `main` — default branch, always stable
- `feature/<name>` — all feature work branches off `main` and PRs back into it

### Branch Naming

- Human-authored branches: `feature/<name>`
- Claude-authored branches: `feature/claude-<name>`

The `feature/claude-` prefix makes it visually clear the branch was Claude's work.

## Release Process

Releases are driven by [release-please](https://github.com/googleapis/release-please)
from the Conventional Commit history. There is one primary path and one
break-glass path.

### Primary path: release-please (no manual tagging)

`.github/workflows/release-please.yml` runs on every push to `main`. It reads the
Conventional Commit subjects since the last release and maintains a standing
**release PR** (`chore(main): release <version>`) that bumps the version and
updates `CHANGELOG.md`.

- **Cutting a release = merging that release PR.** Merging it tags the commit and
  creates the GitHub release, then — in the *same* run — delegates the artifact
  build + e2e to the central reusable workflow
  `rocne/release-ci/.github/workflows/release.yml` (pinned `@v0.1.1`). No PAT is
  needed because the build is chained in-run rather than triggered by the
  API-created tag.
- **The version is derived, not chosen.** `fix:` → patch, `feat:` → minor (while
  pre-1.0, per `bump-minor-pre-major`), `feat!:`/`BREAKING CHANGE` → still minor
  pre-1.0. Commits typed `docs:`/`chore:`/`ci:`/`style:`/`refactor:`/`test:` do
  not trigger a release on their own.
- **PR titles are the source of truth.** PRs are squash-merged, so the PR title
  becomes the commit subject release-please reads. `.github/workflows/pr-title.yml`
  enforces Conventional-Commit PR titles; a malformed title silently drops the
  change from release automation.

State lives in `release-please-config.json` and `.release-please-manifest.json`
(current released version). Do not hand-edit the manifest — release-please owns it.

### Break-glass path: manual tag

`.github/workflows/release-manual.yml` triggers on a pushed `v*` tag and calls the same
central `rocne/release-ci` reusable workflow. Use only when release-please can't (e.g. re-cutting a botched
release). Tag format `v<semver>` (e.g. `v0.6.1`); always tag from `main`, and the
workflow files must exist at the tagged commit.

```sh
git tag v0.6.1
git push origin v0.6.1
```

Both paths funnel through the central `rocne/release-ci` reusable workflow so the
manual and automated releases can never drift: GoReleaser builds linux+darwin ×
amd64+arm64, publishes the GitHub release, bumps the Homebrew tap, then runs
release e2e (opens an issue on failure).

### Repo setting required

release-please needs **Settings → Actions → General → "Allow GitHub Actions to
create and approve pull requests" = ON** (it opens the release PR). This is
currently ON; if release PRs stop appearing, check it first.

## Canonical Resolution Paths

Every value with a canonical source must be obtained through that source. The two chains:

**Paths** (`cfg.initFile`, `cfg.linkRoot`, `cfg.binDir`, etc.) — resolved once in `resolvePaths()` via `ecosystem.ResolvePath`: CLI flag → `DOTD_*` env var → env.yaml field → XDG/system default. Command code reads `cfg.*`; it never calls `ecosystem.DefaultX()` directly or re-queries env vars.

**Env values** (`os`, `shell`, `context`, etc.) — resolved via `resolveEnv(cfg)` which reads env.yaml and applies shell vars and `--env` overrides. Command code reads from the returned map; it never calls `runtime.GOOS`, `os.Getenv("SHELL")`, or equivalent directly.

**Violations to avoid:**
- Calling `ecosystem.DefaultInitFile()` (or any `Default*`) outside of `resolvePaths`
- Reading `os.Getenv("DOTD_*")` in command code after `resolvePaths` has already run
- Using `runtime.GOOS` or `os.Getenv("SHELL")` anywhere outside `cmd/dotd/getters.go`
- Redundant `os.UserHomeDir()` calls in commands where `cfg.linkRoot` is already resolved

**Legitimate exceptions:** `resolvePaths` itself; `getters.go` (these implement the canonical detectors); bootstrap code in `dotd init` that runs before env.yaml exists.

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
