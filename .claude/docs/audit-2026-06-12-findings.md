# Audit Findings — 2026-06-12

> **STATUS (same day):** plan executed. PR #106 (H1), #107 (D1–D5, U4),
> #108 (S1, S2, P4 — plan step 2b dropped deliberately, see PR body),
> #109 (P1, P2, C1), #110 (U1, U2, U3, P3). Remaining open: **P5 only**
> (unapply --all flag shadow — deferred by design).

Categories: **DOCS** (documentation accuracy), **UX** (first-run / interactive
experience), **SCRIPT** (scriptability/automation), **POLISH** (output quality),
**HYGIENE** (repo policy), **CODE** (code quality).

Priorities: **High** (misleads or blocks users), **Medium** (confuses users),
**Low** (cosmetic).

Evidence for every item: `.claude/docs/audit-2026-06-12-trail.md`.

---

## High

| ID | Cat | Finding |
|----|-----|---------|
| D1 | DOCS | README "Commands" section documents a CLI that doesn't exist: `dotd files list`, `dotd link apply/check/remove`, `dotd dag apply`, `setup --yes`, `--verbose` on apply/check/dag, `env set context=work` (real syntax: `env set context work`). README is the front door; ~40% of its command examples fail when pasted. |
| D2 | DOCS | `docs/reference/dotd.md` (published site): whole `dotd link` section for a nonexistent command; `dag apply` examples; `setup --yes`; adopt described as "Copies a file" with nonexistent `--no-interactive` flag and "offers to remove the original" (actual: moves + replaces with symlink); global-flags table lists nonexistent `--verbose`, wrong `--link-root` default (`~/.config`; actual `$HOME`), missing `--config`/`--quiet`/`--log-level`/`--debug`/`--generated-dir`/`--all`. |
| D3 | DOCS | README `.dotd.yaml` config section uses the legacy filename/format; the repo-layout section above it already says `.dagger`. Self-contradictory. (`docs/reference/dotd-yaml.md` correctly marks it legacy.) |
| U1 | UX | No-config fallback walks cwd: on a fresh machine `dotd apply`/`check`/`list` fall through `-f → $DOTD_FILES → $DOTFILES → config.yaml → cwd` and walk whatever directory you're in. From /tmp it scanned unrelated dirs, emitted warnings about other users' files, and died on a permission error with no hint. Should fail fast ("no config — run `dotd setup` or pass `-f`") when there's no explicit source and cwd doesn't look like a dotfiles repo. |
| U2 | UX | Silent no-op trap: active nodes with zero actions (convention dirs missing `.dagger`) make `apply` succeed with "0 symlinks applied … (0 nodes)" while `list`/`check` show the nodes as active. New users hit exactly this (init skips `.dagger` scaffolding when non-TTY). Needs a warning: "N active nodes have no actions — run `dotd init`". |
| S1 | SCRIPT | `dotd init` is not scriptable: no `-y`/`--non-interactive` (setup has `-n` — inconsistent). Non-TTY+EOF silently skips everything (so scripted bootstrap creates no `.dagger`, causing U2). Piped input is a trap: `yes \| dotd init` creates a directory literally named `y` (name prompt consumes the next line) and writes all three sections' `.dagger` into it. |

## Medium

| ID | Cat | Finding |
|----|-----|---------|
| D4 | DOCS | Setup/init responsibility documented wrong: README + `docs/reference/dotd.md` + `docs/getting-started/first-machine.md` say `setup` "scaffolds a dotfiles repo … and wires up your shell". Actual: `setup` only writes config.yaml/env.yaml; `init` scaffolds dirs and appends the RC source line (`maybeAddSourceLine`, init_cmd.go:64). first-machine.md:25 also uses `setup --yes`. |
| D5 | DOCS | `docs/index.md:42` "Each stage is also available standalone: `dotd env`, `dotd dag`, `dotd link`, `dotd package`" — `dotd link` doesn't exist; `dag` only inspects. |
| S2 | SCRIPT | `setup -n` prints section labels and "Accepting shown defaults." but never shows the accepted values. User can't see what was written without opening config.yaml. Print `label: value` per field. |
| U3 | UX | `check` on an unconfigured machine ends with bare `error: check: issues found` (main.go:632) — no hint. When config.yaml is absent, hint should point to `dotd setup`. |
| H1 | HYGIENE | `docs/audit/` (10 files) and `docs/superpowers/` (plans/handoffs/specs) are committed to the public repo but not in mkdocs nav — violates CLAUDE.md "What to Commit" (working docs stay local). **Needs user confirmation before removing from git history-forward (plain `git rm` keeps history; that's fine).** |

## Low

| ID | Cat | Finding |
|----|-----|---------|
| P1 | POLISH | Pluralization: "1 symlinks applied" (main.go:581), "(1 nodes)" (main.go:588). |
| P2 | POLISH | Double-wrapped walk error: `dagger: open <path>: open <path>: permission denied` — dagger.go:82 wraps `os.Open`'s error (which already contains `open <path>:`) with another `open %s:`. |
| P3 | POLISH | `dotd list` / `dotd env show` on empty state print nothing, exit 0. A stderr note ("no nodes found — is this a dotfiles repo?" / "env.yaml not found") would orient users; keep stdout empty for pipes. |
| P4 | POLISH | `init` prompt rendering: `Create this directory? [Y/n]:   › name [shellrc]:` — two prompts joined on one line; non-TTY shows `[Y/n]:   skipping` with odd alignment. |
| U4 | UX/DOCS | `dot-` prefix inside `config/` yields hidden files inside `~/.config` (e.g. `config/dot-tmux.conf` → `~/.config/.tmux.conf`) — surprising; most tools want `~/.tmux.conf` (top level) or `~/.config/tmux/tmux.conf`. Document the interaction in README naming-conventions section. |
| P5 | POLISH | `unapply --all` (remove all symlinks) shadows the root persistent `--all` (show internal commands) — same flag name, unrelated meanings. Works (local wins), but consider renaming one later; at minimum keep help text crisp. |
| C1 | CODE | `internal/annotation/registry.go:103` hardcodes `{"source", "no-source", "link"}` — duplication of `pipeline.Action*` forced by import cycle (pipeline imports annotation). Add a sync-warning comment on both sides, or move action constants to a leaf package both can import. |

## Explicitly checked, no action

- Exit codes (1 runtime / 2 usage) — correct.
- error:/hint: convention — working.
- Cobra suggestions — working.
- Output routing, canonical resolution paths — clean (teardown's direct
  default call is a documented exception).
- Build, vet, tests — all pass.
