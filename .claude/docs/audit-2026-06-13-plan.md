# Code-Smell Remediation Plan — 2026-06-13

STATUS: COMPLETE — all six code PRs merged same-day (#111–#116, releases
v0.4.3–v0.4.8 auto-cut). PR 7 (audit-guide updates) done as local edits.
Only P4 fix-when-touched leftovers and the X decision queue remain — see
the findings doc STATUS block.
- PR 1 → DONE: merged as PR #111 (`5d629c9`), release v0.4.3. Included a
  bonus pipeline fix (Act now reads compose fragments in dry-run) and two
  e2e script updates (compose.sh expected exit 0 on stale; adopt.sh lacked
  --yes). Lesson recorded: tests/e2e had encoded the buggy behaviors.
- Deep internal/ pass added PR 5 and PR 6 below (findings B5–B9, S8–S10,
  D7–D11, C5).
Findings: `audit-2026-06-13-findings.md`. Evidence: `audit-2026-06-13-trail.md`.
Baseline: `main` = `ae944d8`, suite green.

Written to be executable by a weaker model: exact files, exact checks.
Per-PR discipline (same as 2026-06-12): branch `feature/claude-<name>` off
fresh `main`; `go build ./... && go vet ./... && go test ./...` +
`go test -tags=integration ./...`; live smoke where noted; `gh pr view`
before every push; watch CI green before merge. Re-read files after any
checkout/rebase.

X-items (C1 rename, D6 prompt consolidation, teardown scope rule, P5) are
NOT in any PR — they need user decisions first. O1 policy is assumed approved
as recommended (mutation results → stdout); if user rejects, drop PR 3 step 1
and instead document the split.

---

## PR 1 — `feature/claude-safety-consistency` (B1, B2, B3) — P1 behavior

1. **B1 teardown --dry-run**: in `runTeardown` (teardown_cmd.go), after the
   preview block and before the confirmation, add:
   ```go
   if cfg.dryRun {
       return nil
   }
   ```
   (mirrors unapply_cmd.go:125 — preview prints, nothing executes).
2. **B2 adopt non-TTY**: adopt.go:92 — replace silent auto-accept. When
   `!yes && !isTTY(cmd.InOrStdin())`, return
   `&hintError{err: errors.New("adopt: confirmation required"), hint: "pass --yes to adopt non-interactively"}`.
   FIRST grep `adopt` usages in main_test.go / integration_test.go /
   adopt_test.go — existing tests pipe stdin and may rely on auto-accept; add
   `--yes` to those invocations as part of this change.
3. **B3 compose check exit**: compose_cmd.go — when `hasStale`, return
   `errors.New("compose check: stale targets found")` after the summary
   (set `cmd.SilenceErrors`? No — root already silences; keep parity with
   `check: issues found` in main.go:683). Apply to the `--json` path too
   (return the error after encoding). Update the Long example only if wording
   changes.
4. Tests: new cases in main_test.go or a new compose_cmd_test.go —
   (a) `teardown --yes --dry-run` leaves files in place;
   (b) `adopt` with piped stdin and no `--yes` exits non-zero, file not moved;
   (c) `compose check` exits non-zero on stale, zero on clean, both text+json.
5. Smoke (sandbox `$HOME` via `mktemp -d`): scripted setup/init/apply, then
   each of the three fixed paths by hand.

Acceptance: all three new behaviors observable in smoke; suite + integration green.

## PR 2 — `feature/claude-stale-names` (S1, S2) — P1 truth

1. **S2**: replace `conf/` with `config/` in: adopt.go Long (inference table
   + both examples), annotate_cmd.go:29 example, adopter/adopter.go doc
   comments (Inference struct doc, Infer rule list, any others).
   Acceptance: `grep -rn '\bconf/' cmd/ internal/ --include='*.go'` → 0 hits.
2. **S1**: setup_cmd.go:120 → both expressions use `ecosystem.ToolD`:
   ```go
   envContent := fmt.Sprintf("os: $(%[1]s get-os)\nhostname: $(%[1]s get-hostname)\n", ecosystem.ToolD)
   ```
   Check setup_cmd_test.go / integration for exact-content assertions on
   env.yaml and update if they assert the literal.
3. Add the rule to `.claude/docs/audit-guide.md` (What NOT to change /
   conventions section): prose+help may say `dotd`; generated/executed
   content must use `ecosystem.ToolD`.

Acceptance: grep clean; `dotd adopt --help` shows `config/dot-bashrc`.

## PR 3 — `feature/claude-output-channels` (O1, O2, O3, O4, S7, O6) — P2/P3

Needs O1 policy confirmed (see X note above). Steps:

1. **O1**: move mutation summaries to stdout ui helpers:
   - main.go:626,633 (apply links/init.sh lines) → `ui.OKf(cmd.OutOrStdout(), …)`
     (keep the Debugf stage lines on cfg.log).
   - adopt.go:128-132 ("adopted …" lines) → ui.OKf / plain Fprintf to stdout.
   Keep `check`'s per-link warnings on cfg.log (diagnostics). Update tests
   that assert these lines on stderr/log output.
2. **S7 + O2**: add `ui.Hintf(w, format, …)` rendering cyan `hint:` (decide:
   replaces Tipf? — keep both only if both labels used; otherwise alias).
   Root handler main.go:101-107: `ui.Errf(os.Stderr, …)`, `ui.Hintf(os.Stderr, …)`,
   and cancelled via `ui.Skipf(os.Stderr, "cancelled")`. Two-space alignment
   goes away — single space from helpers; update any tests matching
   `hint:  `. filter_prompt.go:63 → ui.Hintf.
3. **O4**: adopt.go:101 cancel → same rendering as promptConfirm
   (ui.Skipf "cancelled"); decide stdout vs stderr once and use for all three
   cancel paths (recommend stderr — status, keeps pipes clean — but
   promptConfirm currently writes stdout; pick one, update tests).
4. **O3**: compose check — per-file lines and summary both via ui to errOut
   (it's a check report, stderr keeps stdout JSON-clean), drop cfg.log there.
5. **O6**: predicate/eval.go — delete `Mode`, `Warn`, `SetWarnOutput`,
   `warnOut`, the ui import; `Call` returns error for unknown function
   (current Strict behavior). FIRST check `.claude/docs/spec/predicates.md`
   for Warn-mode spec language — if specced, ask user instead of deleting.
   Update eval tests that exercise Warn mode.
6. **O5** opportunistically: while in teardown/init/setup, unify skip
   vocabulary ("skip %s" / "exists %s") and the two-space indent.

Acceptance: `grep -rn 'fmt\.Fprintf(os\.Stderr' cmd/dotd/` shows only
launchEditor wiring; no `cfg.log` mutation-result lines; suite green;
smoke shows colored error:/hint: on a failing command.

## PR 4 — `feature/claude-dedup-pass` (D1/B4, D2, D3, D5, S3, S4, S5, S6, C4, D4) — P2/P3

1. **D1+B4**: extract shared preamble in main.go:
   ```go
   // walkActive: guard → resolveEnv → Walk(+debug-log disabled) →
   // ValidateNodes(all) → filterWithPrompt → Order
   func walkActive(cmd *cobra.Command, cfg *config) (ordered []pipeline.RawNode, resolvedCount, allCount int, err error)
   ```
   Semantic decision (recommended): validate ALL nodes pre-filter in both
   paths — read commands gain the stricter check apply already has.
   `runPipeline` = walkActive + Act. `walkOrdered` becomes walkActive.
   Watch for double-prompt regressions (filterWithPrompt runs once per
   command invocation — unapply --all path bypasses, unchanged).
2. **S4**: move `plural` from config_cmd.go to new `cmd/dotd/format.go`;
   replace `%d symlink(s)` at unapply_cmd.go:113,115 and teardown_cmd.go:53
   with `plural(...)`. Acceptance: `grep -rn '(s)' cmd/dotd/ --include='*.go' | grep -v _test` → no message hits.
3. **D3**: add `keyArgs(nArgs int, usage, hint string) cobra.PositionalArgs`
   to errors.go; configKeyArgs/envKeyArgs become one-line callers or die.
4. **D2**: entries-first rendering in env diff (env.go), package list+check
   (package.go): build the existing entry slice unconditionally, then
   `if jsonOutput { encode } else { text loop }` reading entries.
5. **S3**: delete `env.DefaultPath`; main.go:333 and teardown_cmd.go:69 call
   `ecosystem.DefaultEnvFile`.
6. **S5**: delete `envYamlPath` (use `cfg.envFile`) and `loadConfig`
   (use `dotcfg.Load`).
7. **S6**: `const BinPrefix = "~bin"` in pipeline (act.go); use in
   expandDest and init_cmd.go:161 template
   (`"link_root: \"" + pipeline.BinPrefix + "\"\n…"`).
8. **C4**: add `bannerf(out io.Writer, cmd *cobra.Command, subtitle string)`
   (cmd/dotd/format.go) rendering `ui.Header(cmd.CommandPath()) + " — " + subtitle`;
   collapse setup_cmd.go:65-78 switch (subtitle chosen from isUpdate/
   nonInteractive) and init_cmd.go:61-65.
9. **D4** opportunistic: `addJSONFlag(cmd *cobra.Command) *bool` helper used
   by the 8 sites (desc stays "output JSON array").
10. **D5**: init_cmd.go:51 → `!fileutil.Exists(cfg.configPath)`; sweep other
    `os.IsNotExist` (none expected); leave `errors.Is(fs.ErrNotExist)` where
    the error value is consumed.

Acceptance greps:
```
grep -rn 'os\.IsNotExist' cmd/ internal/ --include='*.go' | grep -v _test  # 0
grep -rn 'DefaultPath' internal/env cmd/                                   # 0
grep -rn '"~bin"' cmd/                                                     # 0
```
Full suite + integration green; `dotd list`/`dag check`/`apply` smoke output
unchanged except intended.

## PR 5 — `feature/claude-predicate-correctness` (B5, B6, B7, B8) — deep-pass behavior

1. **B5**: walk.go:150 — wrap the compose-dir's own when before combining:
   ```go
   dirWhen := cfg.When
   if dirWhen != "" {
       dirWhen = "(" + dirWhen + ")"
   }
   effectiveWhen := combineWhen(state.when, dirWhen)
   ```
   Regression test: compose dir `when: "a=1 OR b=2"` under ancestor default
   `when: "c=3"`; env `b=2, c≠3` must NOT activate the target.
2. **B6**: annotation/registry.go WhenType.Validate → delegate:
   ```go
   func (WhenType) Validate(s string) error {
       if _, err := predicate.Parse(s); err != nil {
           return fmt.Errorf("@when: %w\nhint: ...", err)
       }
       return nil
   }
   ```
   Tests: `exists(tmux)` accepted; `os=a AND` rejected.
3. **B7**: read `.claude/docs/spec/predicates.md` for installable() semantics
   FIRST. Then load packages.yaml registry in the pipeline preamble and call
   `ev.RegisterPackageRegistry(reg, nil)` — likely means evalWhen/Filter gain
   a registry parameter or the Evaluator is constructed once per
   filter run (touches filterWithPrompt + Filter + CollectMissingKeys callers).
   Test: `@when(installable(x))` true with registry entry + manager on PATH.
4. **B8**: first write the reproducing test (files: entry with inherited
   link_root and no explicit dest → expect derived link). If it fails,
   populate LinkRootDir/IsCompose/ComposeTarget in the files:-dict RawNode
   (walk.go:304-313) from `state`.

## PR 6 — `feature/claude-internal-hygiene` (D7, D8, D9, D10, D11, S8, S9, S10, B9, C5)

1. **D7**: add `fileutil.WriteAtomic(path string, data []byte, mode os.FileMode)`;
   rewrite SaveYAML on top of it; initgen.writeAtomic and annotation.Write
   delegate; act.go generated-file write uses it (mode fileutil.ModeFile).
2. **S8**: `const sourceLineHeader = "# dotd — generated shell init"` in
   setup/shell.go; Append + Remove both use it.
3. **D8**: delete env.parseFlags; delete packages.Catalog (CHECK spec/docs
   references first — `grep -rn Catalog docs/ .claude/docs/spec/`); reconcile
   the two adopt dry-run layers (keep adopter.DryRun support, have cmd pass
   it through instead of returning early — or delete the adopter field;
   prefer whichever keeps one owner); export packages.EmptyRegistry() and use
   in predicate/builtins.go.
4. **B9**: InstallCmd substitutes PlaceholderToken on the override path too.
5. **S9**: move the predicate syntax table to one exported const (predicate
   package); registry.go + concepts_cmd.go reference it.
6. **S10**: one generated-header helper (suggest in fileutil or ecosystem:
   `ecosystem.GeneratedHeader(tool string)`); align `package generate` help
   with the script's sudo note.
7. **D9**: single shell-quote helper (move initgen.singleQuote to fileutil;
   bundle.go uses it — keep the conditional-quoting behavior only if tests
   depend on it; otherwise always-quote).
8. **D10/C5**: opportunistic — consolidate manager loops; debug-log unknown
   @after refs in order.go.

## PR 7 — `feature/claude-audit-guide-update` (docs only, optional fold-in)

Fold into whichever PR lands last if preferred. Update
`.claude/docs/audit-guide.md` (NOTE: audit-guide is a local working doc —
keep out of git per commit-scope rule; this "PR" is local-only edits):
- ToolD rule (from PR 2)
- O1 channel policy as ratified
- error-prefix convention paragraph (C3)
- new audit lenses used this session (advertised-vs-honored flags,
  entries-first rendering shape, prompt-stack comparison)

## Deferred / decision queue

| Item | Why deferred |
|------|--------------|
| C1 `dag check` rename | breaking command change — user call |
| D6 prompt-stack consolidation | spec-level; affects test driving patterns |
| teardown scope rule (X4) | needs a decision on override semantics |
| P5 `unapply --all` shadow | carried from 2026-06-12 audit |
| C2 alias policy, C3 stragglers, O5 leftovers | fix when touching those files |

## Suggested order

~~PR 1~~ (done, #111) → PR 2 → PR 5 (behavior, high value) → PR 3 → PR 4
(rebase after 3; both touch main.go) → PR 6 → PR 7 fold-in. Auto-release
fires per merge (internal/** and cmd/dotd/** paths) — v0.4.3 already cut for
PR 1; expect more patch tags.

## Execution lessons (PR 1)

- Tests and e2e scripts may encode the very bug being fixed — grep
  test/e2e/*.sh and *_test.go for the old behavior before assuming CI green
  means done (compose.sh asserted exit-0-on-stale; TestAct_Compose_DryRun
  asserted empty dry-run content; adopt.sh relied on non-TTY auto-accept).
- e2e runner stops at first failing script: one red script can mask others
  (adopt.sh never ran in the first failed CI round). Sweep all scripts for
  affected commands preemptively.
