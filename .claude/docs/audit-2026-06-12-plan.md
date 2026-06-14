# Remediation Plan — 2026-06-12 audit

Executes the findings in `audit-2026-06-12-findings.md`. Five PRs, ordered by
value/risk. Each PR section lists: branch name, exact steps, and acceptance
checks. Follow the steps literally; where a step says "verify against
`--help`", the binary's help output is the source of truth — never the old
docs text.

**Ground rules (from CLAUDE.md — apply to every PR):**
- Branch off `main`, name `feature/claude-<name>`. Never commit to `main`.
- Before every push: `gh pr view` — if the PR is merged, make a new branch.
- Validate before committing: `go build ./... && go vet ./... && go test ./...`
  (for docs-only PRs, build/test still must pass — they prove nothing broke).
- Commit messages end with the Claude Code co-author trailer.

**How to check actual CLI behavior** (needed in PR 1):
```sh
go build -o /tmp/dotd ./cmd/dotd
/tmp/dotd --help
/tmp/dotd <command> --help      # for every command you document
```

---

## PR 1 — Docs truth pass (D1 D2 D3 D4 D5 U4) — HIGH, do first

Branch: `feature/claude-docs-truth-pass`. Docs-only; no Go changes.

Rewrite every command example so it matches the real CLI. The real CLI is:
commands `adopt annotate apply check list unapply config env init setup
teardown bundle compose dag package completion concepts` (plus hidden
`get-hostname get-os`). There is **no** `files`, **no** `link`, **no**
`--verbose` flag anywhere, **no** `dag apply`. `dag` has only `check`.

### 1a. README.md

- "How it works" (~line 110): replace
  `Each stage is also available standalone: dotd env, dotd dag, dotd link, dotd package.`
  with: `Some stages are inspectable standalone: dotd env, dotd dag check, dotd package, dotd compose.`
- Quick start (~167–182): remove `--verbose` example lines. Replace the manual
  `echo 'source …' >> ~/.zshrc` lines with: `dotd setup` then `dotd init`
  (init offers to append the source line). Keep `--dry-run` example.
- `### dotd setup` (~239): change `dotd setup --yes` → `dotd setup --non-interactive`
  (alias `-n`). Fix description: setup writes config.yaml/env.yaml only; it
  does **not** scaffold the repo or wire the shell — `dotd init` does that.
  Mention the two-step flow: `dotd setup` → `dotd init`.
- `### dotd adopt` (~249): change "Copies an existing file into your dotfiles
  repo" → "Moves a file into your dotfiles repo and replaces the original
  with a symlink." Keep inference table (it is correct).
- `### dotd files` (~270–278): delete the whole section. Replace with a
  `### dotd list` section:
  ```sh
  dotd list                      # list active nodes for this machine
  dotd list --json               # machine-readable
  dotd list --env os=macos       # preview for a different environment
  ```
  (Verify exact flags with `/tmp/dotd list --help` and adjust.)
- `### dotd env` (~281): change `dotd env set context=work` →
  `dotd env set context work` (positional key value, no `=`).
- `### dotd dag` (~290): delete `dag apply` lines and `--verbose` line. Keep:
  ```sh
  dotd dag check                 # print nodes in dependency order
  dotd dag check --json
  ```
  (Verify with `/tmp/dotd dag check --help`.)
- `### dotd link` (~300): delete the whole section. Symlinks are managed by
  `apply` / `check` / `unapply`; say that in one sentence under `### dotd apply`
  or add a short `### dotd unapply` section mirroring `docs/reference/dotd.md`.
- `### .dotd.yaml` (~511–527): retitle to `### .dagger` and update the example
  to the current format. Copy the canonical example from
  `docs/reference/dagger.md` (that file is current). Add one line: ".dotd.yaml
  is the legacy name; rename to .dagger."
- Naming conventions (~200): add a note for U4: a `dot-` prefixed file inside
  `config/` becomes a *hidden file inside* `~/.config`
  (`config/dot-tmux.conf` → `~/.config/.tmux.conf`); for a home-level dotfile
  use `@symlink ~/.tmux.conf` instead.

### 1b. docs/reference/dotd.md

- Global flags table (lines 9–19): regenerate from `/tmp/dotd --help`. Remove
  `--verbose`. Fix `--link-root` default to `$HOME` (config.yaml `link_root`
  overrides). Add `--config`, `--quiet`, `--log-level`, `--debug`,
  `--generated-dir`, `--all`.
- `## dotd setup` (60–71): `--yes` → `--non-interactive`/`-n`; fix the
  description the same way as README (setup = config files only).
- `## dotd adopt` (119–148): "Copies" → "Moves … and replaces the original
  with a symlink". Flags table: keep `--to`, `--yes/-y`; **delete**
  `--no-interactive` (doesn't exist). Delete "offers to remove the original"
  sentence; say: the original is replaced by a symlink; run with `--dry-run`
  to preview.
- `## dotd dag` (176–189): delete all `dag apply` lines; document only
  `dag check` (+ `--json`).
- `## dotd link` (211–233): delete the section. Move the "Symlink states"
  table into `## dotd check` (the states appear in check output).
- `## dotd init` (85–93): add that init also offers to append the shell
  source line (this is where wiring happens, not setup).

### 1c. docs/index.md

- Line 42: same fix as README "How it works" standalone-stages sentence.

### 1d. docs/getting-started/first-machine.md

- Line 25: `dotd setup --yes` → `dotd setup --non-interactive`.
- Line ~103: change "If `dotd setup` didn't append the source line" → "If
  `dotd init` didn't append the source line".
- Read the whole file; fix any other `--verbose`, `files`, `link`, `dag apply`
  occurrences the same way.

### Acceptance (PR 1)

```sh
# zero hits for stale tokens across user-facing docs:
grep -rn -- "--verbose\|dotd files\|dotd link \|dag apply\|setup --yes\|--no-interactive\|env set [a-z]*=" README.md docs/ --include="*.md" | grep -v dotd-yaml.md | grep -v audit/ | grep -v superpowers/
# (expect: no output)
go build ./... && go test ./...
```
Then paste every command example from README into a shell with a sandbox HOME
and confirm none errors with "unknown command/flag".

---

## PR 2 — init scriptability + setup -n output (S1 S2 P4) — HIGH

Branch: `feature/claude-init-noninteractive`.

### 2a. Add `-y, --yes` to `dotd init` (S1)

File: `cmd/dotd/init_cmd.go`.
- Add flag: `cmd.Flags().BoolP("yes", "y", false, "accept all defaults without prompting")`
  (match how `setup` registers `--non-interactive` in setup_cmd.go; mirror that
  mechanism exactly — if setup uses a struct field + `-n`, consider naming this
  `--non-interactive`/`-n` for consistency with setup instead of `-y`. Pick
  **`-n/--non-interactive`** to match setup; `unapply`/`adopt`/`teardown` use
  `-y` for *confirmation* prompts, but init is a *wizard* like setup).
- When set: skip `promptYN` ("Create this directory?") treating answer as yes,
  and skip the name prompt treating answer as the shown default
  (`shellrc`/`config`/`bin`). Also auto-accept the `maybeAddSourceLine`
  prompt's default.
- Print each accepted value, same style as the fix in 2c.

### 2b. Fix the piped-stdin trap (S1)

Current bug: `yes | dotd init` creates a dir named `y` because the name
prompt reads the next stdin line. Fix: in the name prompt inside
`cmd/dotd/init_cmd.go` (the `› name [shellrc]:` prompt — find via
`grep -n "name \[" cmd/dotd/init_cmd.go` region near line 156), validate the
entered name: reject names not matching `^[A-Za-z0-9._-]+$`? **No — simpler
and sufficient:** when stdin is not a TTY (the existing TTY check used by
`promptYN` — see `cmd/dotd/prompts.go`), do not read free-text input; use the
default name. Free-text prompts in non-TTY mode silently taking the default
is consistent with how `setup -n` behaves.

### 2c. `setup -n` prints accepted values (S2)

File: `cmd/dotd/setup_cmd.go`. In non-interactive mode, after each section
(or where the value is resolved), print `  <label>: <value>` via the
existing out writer, e.g.:
```
  Dotfiles repo: /home/user/dotfiles
  Bin directory: /home/user/.local/bin/dot-dagger
```
Use the same indentation as the existing section labels. Interactive mode
already shows values in prompts — change non-interactive output only.

### 2d. Prompt rendering (P4)

In `init_cmd.go`, ensure the `[Y/n]` prompt and the name prompt render on
separate lines (add `\n` after the Y/n answer is consumed), and the non-TTY
"skipping" note prints as its own aligned line.

### Tests (use existing helper `runWithStdin(t, reader, args...)` in cmd/dotd/main_test.go)

- `TestInit_NonInteractiveFlag`: setup -n first, then `init -n` with `nil`
  stdin → all three dirs + `.dagger` files exist; RC prompt not blocking.
- `TestInit_PipedYes`: stdin = `strings.NewReader("y\ny\ny\n")` (no -n flag) →
  no directory named `y` is created; defaults used for names.
- `TestSetup_NonInteractivePrintsValues`: output contains `Dotfiles repo:` and
  the resolved path.

### Acceptance (PR 2)

```sh
go test ./cmd/dotd/
HOME=$(mktemp -d); export HOME
/tmp/dotd setup -n --files $HOME/dotfiles && /tmp/dotd init -n
ls $HOME/dotfiles/shellrc/.dagger $HOME/dotfiles/config/.dagger $HOME/dotfiles/bin/.dagger  # all exist
[ ! -e $HOME/dotfiles/y ]   # no 'y' dir
```

---

## PR 3 — first-run guardrails (U1 U2 U3 P3) — HIGH/MEDIUM

Branch: `feature/claude-first-run-guardrails`.

### 3a. Fail fast instead of walking cwd (U1)

File: `cmd/dotd/main.go`, `resolvePaths` (~line 340–346). The dotfiles-path
chain currently ends in a cwd fallback (`ecosystem.DefaultDotfiles()`).
- Record provenance: add a bool field to `config` (e.g.
  `filesFromCwdFallback`) set true only when the cwd fallback was used
  (no `-f`, no `$DOTD_FILES`/`$DOTFILES`, no config.yaml `dotfiles` value).
- In the pipeline entry point used by walk-based commands
  (`runPipeline` in main.go — confirm via `grep -n "func runPipeline" cmd/dotd/main.go`):
  if `cfg.filesFromCwdFallback` **and** config.yaml does not exist on disk,
  return a `hintError`:
  - error: `no dotfiles repo configured (would walk current directory)`
  - hint: `run 'dotd setup', or pass -f <path> / set $DOTFILES`
- Do **not** error when config.yaml exists (user configured but left
  `dotfiles` empty intentionally — preserve current behavior) or when the
  user explicitly passed `-f .`. This keeps the change reversible and scoped
  to the true fresh-machine case.

### 3b. Warn on active-but-actionless nodes (U2)

After the pipeline act stage in `apply` (main.go ~575–590), compute:
active node count (already available as filter count) and the number of
nodes that produced *any* action (links + sourced + composed). If
`active > 0` and all three are zero, log a warning:
```
warning: N active node(s) produced no actions — convention dirs may be missing .dagger files; run 'dotd init'
```
Use `cfg.log.Warnf` (or `ui.Warnf` to match existing warnings — copy the
mechanism used by the "nosync- path not gitignored" warning, found via
`grep -rn "not gitignored" internal/ cmd/`).

### 3c. `check` hint when unconfigured (U3)

File: `cmd/dotd/main.go:632` (`return errors.New("check: issues found")`).
When config.yaml is absent, return a `hintError` instead (see
`cmd/dotd/errors.go` for the type):
- error: `check: issues found`
- hint: `no config.yaml found — run 'dotd setup' to configure this machine`
When config.yaml exists, keep current behavior.

### 3d. Empty-state notes (P3)

- `dotd list` with zero nodes: print `no nodes found` to **stderr** (logger
  info or `ui.Skipf` to err writer) — stdout must stay empty for pipes. Skip
  the note when `--json` (emit `[]` as now).
- `dotd env show` when env.yaml missing: same pattern — stderr note
  `env.yaml not found — run 'dotd setup'`, stdout empty, `--json` unchanged.

### Tests

- `TestApply_NoConfigNoFiles_FailsFast`: temp HOME, no config, cwd = temp dir
  → exit error contains "no dotfiles repo configured" and hint line.
- `TestApply_WarnsWhenNoActions`: repo with one file in `shellrc/` and no
  `.dagger` → apply succeeds, stderr/log contains "produced no actions".
- `TestCheck_HintsSetupWhenUnconfigured`: error includes the setup hint.
- Update any e2e tests that relied on cwd-fallback behavior (search
  `test/` and `cmd/dotd/integration_test.go` for tests running apply without
  `-f`; pass explicit `-f` where they were relying on cwd).

### Acceptance (PR 3)

```sh
go test ./... && go test -tags integration ./cmd/dotd/
HOME=$(mktemp -d) sh -c 'cd /tmp && /tmp/dotd apply; test $? -eq 1'   # fails fast, no walk of /tmp
```

---

## PR 4 — polish (P1 P2 C1) — LOW, small mechanical PR

Branch: `feature/claude-output-polish`.

### 4a. Pluralization (P1)

`cmd/dotd/main.go:581` and `:588` (and `:627`, `:577` for "nodes"):
add/use a tiny helper in `cmd/dotd` (or `internal/ui` if one exists — check
`grep -rn "func plural" internal/ cmd/` first):
```go
func plural(n int, word string) string { if n == 1 { return word }; return word + "s" }
```
Render `1 symlink applied`, `(1 node)`, `2 symlinks applied`, `(0 nodes)`.
Update tests asserting the old strings (`grep -rn "symlinks applied" cmd/ test/`).

### 4b. Double-wrapped open error (P2)

`internal/dagger/dagger.go:82`:
```go
return nil, fmt.Errorf("dagger: open %s: %w", path, err)
```
→ `os.Open`'s `*PathError` already renders `open <path>: …`. Change to:
```go
return nil, fmt.Errorf("dagger: %w", err)
```
Check `dagger_test.go` for assertions on the old format and update.

### 4c. Registry duplication comment (C1)

`internal/annotation/registry.go:103` and `internal/pipeline/walk.go` (where
`ActionSource`/`ActionNoSource`/`ActionLink` are declared): add a paired
comment on both sites:
```go
// NOTE: must stay in sync with pipeline.Action* (internal/pipeline/walk.go).
// annotation cannot import pipeline (import cycle).
```
Do **not** restructure packages in this PR.

### Acceptance (PR 4)

`go test ./...`; run `apply` on a one-file repo and confirm `1 symlink applied (1 node)`.

---

## PR 5 — repo hygiene (H1) — MEDIUM, **requires user confirmation first**

Branch: `feature/claude-untrack-working-docs`. **Ask the user before
executing** — this removes files from the public repo (history retains them).

```sh
git rm -r --cached docs/audit docs/superpowers
mkdir -p .claude/docs/archive
git mv … # no — files are now untracked; just move on disk:
mv docs/audit .claude/docs/archive/audit-2026-05
mv docs/superpowers .claude/docs/archive/superpowers
printf '/docs/audit/\n/docs/superpowers/\n' >> .gitignore   # optional guard
```
Confirm `mkdocs build` still succeeds (nav never referenced these). Commit
message should state files move to local `.claude/docs/archive/` per
CLAUDE.md commit policy.

### Acceptance (PR 5)

`git ls-files docs/ | grep -E "audit|superpowers"` → empty; `mkdocs build` OK
(or `pip install -r docs/requirements.txt` first).

---

## Suggested order

1 (docs) → 2 (init) → 3 (guardrails) → 4 (polish) → 5 (hygiene, after user OK).

PRs 1, 2, 4 are independent and could go in parallel branches off `main`.
PR 3 should land after PR 2 (its tests use `init -n`).
