# dot-dagger Audit Remediation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the 68 confirmed findings from `docs/audit/` in a safe, dependency-aware order — correctness bugs first, a test safety net before the refactors that touch untested code, then structural cleanup and test backfill.

**Architecture:** Nine workstreams (W1–W9), sequenced. W1 fixes the one Critical correctness bug. W2 builds characterization tests over the highest-risk untested pipeline code (compose, link derivation, symlink safety) so the W3 parser/dedup refactors can't regress silently. W4–W7 are structural (shared constants, ownership, API surface, UX). W8 backfills remaining test gaps. W9 closes the e2e/CI hole. Each AUDIT-NNN maps to exactly one task; cross-references are noted.

**Tech Stack:** Go 1.22, cobra, charmbracelet/huh, gopkg.in/yaml.v3. Tests: standard `go test` + `go tool cover`. e2e: bash + Docker.

**Working rule for every code-change task that touches code currently <80% covered:** write the characterization test FIRST (it should pass against current behavior), then refactor, then confirm the test still passes. This is how the refactors in W3/W5/W6 stay safe.

**Source of truth:** `docs/audit/01..07-*.md` (full finding text), `docs/audit/summary.md` (overview), `docs/audit/review-log.md` (verdicts). Read the cited finding before starting each task.

**Severity counts:** 3 Critical, 22 High, 26 Medium, 17 Low.

**Verification baseline (run once before starting):**
```bash
cd /home/rocne/git/dot-dagger
go build ./... && go test ./... 2>&1 | tail -20
```
Expected: builds clean, all non-skipped tests pass. Record the count of skipped tests (`go test ./... 2>&1 | grep -c SKIP` ≈ 7) — W2/W8 will reduce it.

---

## W1 — Critical correctness: path resolution (AUDIT-001, -002, -003)

The keystone workstream. AUDIT-001 is the only Critical correctness bug; -002/-003 are the structural asymmetry that makes -003 a latent landmine once -002 is fixed. Do all three together.

### Task 1.1: Fix empty `linkRoot` default so all consumers see the same `$HOME` (AUDIT-001, Critical)

**Files:**
- Modify: `cmd/dotd/main.go:222` (the no-op default fn in `resolvePaths`)
- Verify-touch: `cmd/dotd/main.go:257-265` (`buildActOptions`), `cmd/dotd/adopt.go:113`, `cmd/dotd/teardown_cmd.go:75`
- Test: `cmd/dotd/main_test.go`

- [ ] **Step 1 — characterization test (currently failing) for `adopt` with default link root.** Add a test that runs `dotd adopt <file>` with no `--link-root` set and asserts it does NOT return `act: HomeDir is required`. Run it; expect FAIL (reproduces the bug).
- [ ] **Step 2 — fix the default.** In `resolvePaths`, replace the no-op `func() (string, error) { return "", nil }` passed for `linkRoot` with the real `$HOME` default (`ecosystem.DefaultLinkRoot()` if it exists, else `os.UserHomeDir`). After this, `cfg.linkRoot` is non-empty in the common case, resolved through the same `ResolvePath` chain (flag → `DOTD_LINK_ROOT` → config → `$HOME`) as every other path.
- [ ] **Step 3 — remove now-redundant downstream re-derivation.** With `cfg.linkRoot` populated, delete the `os.UserHomeDir()` fallback in `buildActOptions` (main.go:257-265) and pass `cfg.linkRoot` directly; confirm `adopt.go:113` and `teardown_cmd.go:75` now pass the populated value. (Honors CLAUDE.md: consumers read `cfg.*`, never re-derive.)
- [ ] **Step 4 — teardown regression test.** Add a test that `teardown` resolves the RC file under the configured link root, not cwd.
- [ ] **Step 5 — run & commit.** `go test ./cmd/... ./internal/pipeline/... ./internal/adopter/...`; expect PASS. Commit: `fix(config): resolve linkRoot $HOME default in resolvePaths (AUDIT-001)`.

### Task 1.2: Route `configPath` through `ResolvePath`; add `--config` / `DOTD_CONFIG_FILE` (AUDIT-002, High)

**Files:**
- Modify: `cmd/dotd/main.go:201` (replace direct `dotcfg.DefaultPath()`), flag registration near other persistent flags
- Reference: `internal/config/config.go:24-26`, `internal/ecosystem/ecosystem.go` (`ResolvePath`, `DefaultConfigFile`)
- Test: `cmd/dotd/main_test.go` (or `config_test.go`)

- [ ] **Step 1 — test the override.** Write a test asserting `--config /tmp/x.yaml` (and `DOTD_CONFIG_FILE`) makes `cfg.configPath` resolve to that path. Run; expect FAIL (no flag/env tier today).
- [ ] **Step 2 — wire it.** Resolve `cfg.configPath` via `ecosystem.ResolvePath` with envVar `DOTD_CONFIG_FILE`, the new `--config` flag, and default `ecosystem.DefaultConfigFile`, mirroring how `envFile`/`initFile` are resolved. Register a persistent `--config` flag.
- [ ] **Step 3 — run & commit.** `go test ./cmd/...`; expect PASS. Commit: `feat(config): make config.yaml path overridable via --config/DOTD_CONFIG_FILE (AUDIT-002)`.

### Task 1.3: Make `setup`/`init` read resolved `cfg.configPath` (AUDIT-003, Medium — depends on 1.2)

**Files:** Modify `cmd/dotd/setup_cmd.go:45`, `cmd/dotd/init_cmd.go:36`

- [ ] **Step 1 — replace re-derivation.** In both, delete the `dotcfg.DefaultPath()` call and read `cfg.configPath` (guaranteed populated by `PersistentPreRunE` → `resolvePaths`, main.go:73-74). Fix init_cmd's inaccurate "runs before any config is loaded" comment.
- [ ] **Step 2 — test.** Add a test that `init`/`setup` honor a `--config`-overridden path (would have written to the default before 1.2+1.3).
- [ ] **Step 3 — run & commit.** `go test ./cmd/...`. Commit: `refactor(cmd): setup/init read resolved cfg.configPath (AUDIT-003)`.

---

## W2 — Pipeline test safety net + un-skip compose (AUDIT-037, -038, -042, -047, -048, -049, -050, -054, -055)

Build coverage over the highest-risk untested pipeline code BEFORE the W3 refactors touch it. These are TDD-natural: write the missing test, watch it pass (or reveal a bug). Two Criticals (037, 038) live here.

### Task 2.1: Compose end-to-end coverage; delete stale skips (AUDIT-037, Critical; cross-ref AUDIT-013)

**Files:**
- Test: `internal/pipeline/act_test.go`, `internal/pipeline/walk_test.go`, `cmd/dotd/integration_test.go:452,472,495,509,517`
- Add fixture: a `.dagger` with a `compose` action + fragments
- Under test: `internal/pipeline/act.go:90-137`, `internal/pipeline/walk.go:116-160,305`, `cmd/dotd/compose_cmd.go`

- [ ] **Step 1 — verify the skips are stale.** Read the five `t.Skip("...not yet implemented in v2...")` lines; confirm `Act` (act.go:90) handles `ActionCompose` and `compose_cmd.go` is wired. (review-log already confirmed; re-confirm before deleting.)
- [ ] **Step 2 — pipeline-level test.** In `act_test.go`, add a test driving `Act` through the compose branch: multiple fragments → assert assembled output order, generated filename (`ComposeFileName`), synthetic source node present, generated file symlinked. In `walk_test.go`, add a `files`/dir `.dagger` fixture with `compose` so `parseDaggerActions` (0%) executes.
- [ ] **Step 3 — un-skip integration tests.** Remove the five `t.Skip` calls. Run `go test ./cmd/... -run Compose -v`; fix any real failures the now-live tests surface (those are bugs, not test problems).
- [ ] **Step 4 — confirm coverage moved.** `go test ./internal/pipeline/... -coverprofile=/tmp/c.out && go tool cover -func=/tmp/c.out | grep -E 'Act|parseDaggerActions|ComposeFileName'`. Expect each well above its audited 0–42.9%.
- [ ] **Step 5 — commit.** `test(compose): add end-to-end coverage, remove stale skips (AUDIT-037)`.

### Task 2.2: `deriveLinkDest` empty-dest derivation (AUDIT-038, Critical)

**Files:** Test `internal/pipeline/act_test.go`; under test `internal/pipeline/act.go:206-223`

- [ ] **Step 1 — test the rewrite rules.** Add an `Act` test with a link node whose `Dest` is empty and `LinkRoot != ""`. Assert `dot-foo` → `.foo`, `nosync-bar` stripping, and correct multi-component join. Run; if it fails, you found a data-corruption bug — fix `deriveLinkDest`.
- [ ] **Step 2 — commit.** `test(pipeline): cover deriveLinkDest dot-/nosync- rewriting (AUDIT-038)`.

### Task 2.3: `createSymlink` force / clobber-guard safety (AUDIT-048, High)

**Files:** Test `internal/pipeline/act_test.go`; under test `internal/pipeline/act.go:245-...`

- [ ] **Step 1 — three-case test.** Add tests for: (a) re-link over existing symlink, (b) `Force` overwrites a real file, (c) **without** `Force`, an existing real file is NOT clobbered and `Act` errors. Case (c) is safety-critical. Run; fix any divergence.
- [ ] **Step 2 — commit.** `test(pipeline): cover createSymlink force/no-clobber paths (AUDIT-048)`.

### Task 2.4: `~bin` link destination expansion (AUDIT-047, High)

**Files:** Test `internal/pipeline/act_test.go`; under test `internal/pipeline/act.go:227,234-241`

- [ ] **Step 1 — test.** Add `Act` tests for `~bin` and `~bin/x` link dests, asserting they expand against `ActOptions.BinDir`. Run; expect PASS or fix.
- [ ] **Step 2 — commit.** `test(pipeline): cover ~bin destination expansion (AUDIT-047)`.

### Task 2.5: Walk `files:` dict entries (AUDIT-049, High)

**Files:** Test `internal/pipeline/walk_test.go` + fixture; under test `internal/pipeline/walk.go:248-299`

- [ ] **Step 1 — fixture + test.** Add a `.dagger` with a `files:` map (covering `disable`, `name`, `when`, per-file `actions`/`after`, dedup-by-type, missing-file skip). Assert each behavior. Run.
- [ ] **Step 2 — commit.** `test(pipeline): cover .dagger files: dict handling (AUDIT-049)`.

### Task 2.6: Walk `@disable` + `disabled` return slice (AUDIT-054, Medium)

**Files:** Test `internal/pipeline/walk_test.go`; under test `internal/pipeline/walk.go:188-191,256-259`

- [ ] **Step 1 — test.** Add a test that stops discarding the second return value: assert a `@disable`d file is absent from `nodes` AND present in `disabled`. Run.
- [ ] **Step 2 — commit.** `test(pipeline): assert disabled-node reporting (AUDIT-054)`.

### Task 2.7: `mergeActions` two-explicit-links branch (AUDIT-055, Medium; cross-ref AUDIT-014)

**Files:** Test `internal/pipeline/actions_test.go` or `walk_test.go`; under test `internal/pipeline/walk.go:467-478`

- [ ] **Step 1 — test.** Add a case with two explicit link annotations (different dests) on one file; assert `mergeActions` keeps both so `validateNode` can later flag the conflict. Run.
- [ ] **Step 2 — commit.** `test(pipeline): cover two-explicit-link merge handoff (AUDIT-055)`.

### Task 2.8: Order tie-break / `mergeSortedByName` (AUDIT-050, Medium)

**Files:** Test `internal/pipeline/order_test.go`; under test `internal/pipeline/order.go:60,76-77,112`

- [ ] **Step 1 — test.** Add a case where 3+ nodes become ready simultaneously after a shared dependency resolves; assert deterministic alphabetical output across repeated runs. Run.
- [ ] **Step 2 — commit.** `test(pipeline): cover deterministic tie-break ordering (AUDIT-050)`.

### Task 2.9: `package check`/`generate` CLI glue (AUDIT-042, High; cross-ref AUDIT-037)

**Files:** `cmd/dotd/integration_test.go:401,420` (un-skip); under test `cmd/dotd/package.go:133`

- [ ] **Step 1 — verify skip is stale, then un-skip.** Confirm `package check`/`generate` are wired; remove the two `t.Skip`s. Run `go test ./cmd/... -run Package -v`; fix real failures (`loadRegistry`, `collectPackageRequests`, `@require` hard-fail).
- [ ] **Step 2 — commit.** `test(package): cover check/generate command glue, remove stale skips (AUDIT-042)`.

---

## W3 — `.dagger` parsing unification (AUDIT-006, -013, -017, -019)

Now that W2 covers the parsers, consolidate them and extract the key vocabulary. **Do not start until Task 2.1 and 2.5 pass.**

### Task 3.1: Single `.dagger` action parser for dir + file level (AUDIT-013, High; cross-ref AUDIT-006, -017)

**Files:** Modify `internal/pipeline/walk.go:305-324` (`parseDaggerActions`) to delegate to the more capable `parseActionString` (walk.go:406-419)

- [ ] **Step 1 — confirm safety net.** Re-run Task 2.1/2.5 tests; they must be green before refactoring.
- [ ] **Step 2 — unify.** Replace `parseDaggerActions`'s hardcoded `HasPrefix("link(")` chain with calls to `parseActionString` so dir-level `actions:` accepts the same arbitrary `type(dest)` set (with malformed-paren fallback) as file-level. Keep the compose/source/nosource handling.
- [ ] **Step 3 — test.** Add a test that an arbitrary action type at directory level now produces a real Action (previously silently dropped). Run all pipeline tests; expect PASS.
- [ ] **Step 4 — commit.** `refactor(pipeline): unify dir/file .dagger action parsing (AUDIT-013)`.

### Task 3.2: Extract shared `splitParen` helper (AUDIT-017, Medium)

**Files:** New `splitParen(s) (head, body string, ok bool)` — place in `internal/pipeline` (consumed by walk) and have `internal/annotation/annotation.go:77-91` (`parseKeyArgs`) call a shared impl, or duplicate-free via a small shared package if import direction forbids it.

- [ ] **Step 1 — extract & redirect.** Factor the `IndexByte('(')`/`LastIndexByte(')')`/TrimSpace+fallback logic into one function; point `parseActionString` and `parseKeyArgs` at it. (If annotation can't import pipeline, put `splitParen` in a leaf package both import.)
- [ ] **Step 2 — test + run.** Existing annotation + pipeline tests must stay green. Commit: `refactor: share paren-splitter between annotation and pipeline (AUDIT-017)`.

### Task 3.3: Single source for annotation key vocabulary (AUDIT-006, High)

**Files:** `internal/pipeline/walk.go:26-31` (`ann*` consts), `internal/dagger/dagger.go:17-23,30` (yaml tags), `internal/annotation/annotation.go:121` (bare `"when"`)

- [ ] **Step 1 — define one home.** Create exported key-name constants in a leaf package (e.g. `internal/annotation` or a new `internal/keys`): `KeyAfter="after"`, `KeyRequire`, `KeyRequest`, `KeyDisable`, `KeyName`, `KeyWhen`, `KeyAction`. Replace the bare `"when"` literal (annotation.go:121) and the `ann*` consts (walk.go) with these. Document that `dagger.go`'s `yaml:"..."` tags must match (yaml tags can't be const-substituted — add a guard test instead, Step 2).
- [ ] **Step 2 — guard test.** Add a test asserting each `dagger.BasicNode` yaml tag equals the corresponding key constant (reflect over struct tags), so a one-sided rename fails the build's test. Run.
- [ ] **Step 3 — commit.** `refactor: centralize .dagger key vocabulary + tag guard test (AUDIT-006)`.

### Task 3.4: Shared `RawNode.HasCompose()` (AUDIT-019, Low)

**Files:** Add method in `internal/pipeline`; replace `cmd/dotd/compose_cmd.go:117-124`, `internal/pipeline/act.go:82-88`, `internal/pipeline/actions.go:57-66`

- [ ] **Step 1 — add + adopt.** Add `func (n RawNode) HasCompose() bool`; route all three sites through it. Run pipeline + cmd tests. Commit: `refactor(pipeline): single HasCompose predicate (AUDIT-019)`.

---

## W4 — Shared constants for magic values (AUDIT-004, -005, -007, -008, -009, -010, -011, -012)

Mechanical, low-risk, high-value-against-silent-breakage. Each task: introduce one constant/source, replace literals, run tests.

### Task 4.1: `packages.yaml` filename constant (AUDIT-004, High)

- [ ] Add `ecosystem.PackagesFileName` (or `packages.DefaultFileName`); replace literals at `cmd/dotd/package.go:134` and `internal/setup/setup.go:78`. Add a test asserting reader and writer resolve the same path. Commit: `refactor: single packages.yaml filename constant (AUDIT-004)`.

### Task 4.2: `{package}` substitution token constant (AUDIT-005, High)

- [ ] Add `packages.PlaceholderToken = "{package}"`; use it in `internal/packages/packages.go:239` `ReplaceAll`. (The 24 catalog literals in `catalog.go:18-101` are data — add a test that every `Install/Uninstall/Update` template contains the token so a future catalog edit can't omit it.) Commit: `refactor(packages): named {package} token + catalog guard (AUDIT-005)`.

### Task 4.3: Reserved package-key constants (AUDIT-007, High)

- [ ] Define `priority`/`binary`/`check`/`prefer` once as consts; use them in the bare literal (`packages.go:57`) and the `known` map (`packages.go:102`). Add a test that the `known` map equals the set of reserved keys (catches tag/map drift). Commit: `refactor(packages): single source for reserved keys (AUDIT-007)`.

### Task 4.4: Convention dir-name constants (AUDIT-008, High)

- [ ] Define `Shellrc`/`Bin`/`Config` once (e.g. `adopter.ConventionNames` already exists — make it the single source); replace the independent literals at `internal/setup/setup.go:63`, `cmd/dotd/init_cmd.go:77,83,89`, and reconcile `internal/dagger/dagger.go:41-43` yaml tags via a guard test. Commit: `refactor: single source for convention dir names (AUDIT-008)`.

### Task 4.5: `env.yaml` hint constant (AUDIT-009, Medium)

- [ ] Add `ecosystem.EnvFileName` and reference it (or format help strings from it) in the ~15 hint strings across `main.go`, `env.go`, `setup_cmd.go`, `filter_prompt.go`. Commit: `refactor: reference env.yaml name from one constant in hints (AUDIT-009)`.

### Task 4.6: `DOTD_` prefix constant (AUDIT-010, Medium)

- [ ] Define `const dotdPrefix = "DOTD_"` in `internal/env/env.go`; use it in both `HasPrefix` (`:125`) and the slice length (`:128`). Commit: `refactor(env): single DOTD_ prefix constant (AUDIT-010)`.

### Task 4.7: File-mode constants (AUDIT-011, Low)

- [ ] Add `const ModeDir = 0o755; ModeFile = 0o644` (a shared `fileutil` or `ecosystem` home); replace the ~16 literal sites listed in the finding. Commit: `refactor: named file-mode constants (AUDIT-011)`.

### Task 4.8: Shebang constant (AUDIT-012, Low)

- [ ] Add `const POSIXShebang = "#!/bin/sh"`; use in `cmd/dotd/bundle.go:90` and `internal/packages/packages.go:281`. Commit: `refactor: shared POSIX shebang constant (AUDIT-012)`.

---

## W5 — Prompt & exit ownership (AUDIT-016, -018, -022, -023, -035)

### Task 5.1: Return error instead of `os.Exit(1)` from prompt helper (AUDIT-022, High; cross-ref -018, -053)

**Files:** `cmd/dotd/filter_prompt.go:37-39`; callers at the 9 sites listed in the finding; `main.go:26-30`

- [ ] **Step 1 — sentinel + return.** Define a sentinel (e.g. `errUserAborted`) and have `filterWithPrompt` return it on `huh.ErrUserAborted` instead of `os.Exit(1)`. Let it propagate to `main()`, which owns the single exit point; map the sentinel to exit code 1 there (and optionally print "cancelled" to stderr). `grep os.Exit cmd/dotd` should now show only `main.go`.
- [ ] **Step 2 — test.** Add a test that an aborted prompt returns the sentinel (now testable through the call, not a process exit). Run `go test ./cmd/...`.
- [ ] **Step 3 — commit.** `fix(cmd): prompt abort returns error, main owns exit (AUDIT-022)`.

### Task 5.2: Consolidate TTY check via `isTTYStdin` (AUDIT-018, Low)

- [ ] Replace the inline `term.IsTerminal(os.Stdin.Fd())` at `cmd/dotd/adopt.go:92` with `isTTYStdin()` (filter_prompt.go:95-97); drop adopt's direct `charmbracelet/x/term` import if now unused. Run tests. Commit: `refactor(cmd): adopt uses isTTYStdin helper (AUDIT-018)`.

### Task 5.3: Move shared prompt primitives into `prompts.go` (AUDIT-023, Medium; cross-ref -035)

**Files:** Move `promptDefault`/`promptYN`/`expandTildeStr` (from `init_cmd.go`) and `printField`/`fieldPrompt` (from `setup_cmd.go`) into `cmd/dotd/prompts.go`

- [ ] **Step 1 — relocate.** Move the five primitives into `prompts.go` (their natural home — it already owns `promptConfirm`); update references in both command files. Pure move, no behavior change. Run tests. Commit: `refactor(cmd): consolidate prompt primitives in prompts.go (AUDIT-023)`.

### Task 5.4: Unify confirm-prompt default + non-TTY behavior (AUDIT-035, Medium)

**Files:** `cmd/dotd/prompts.go` (`promptConfirm` `[y/N]`), `cmd/dotd/init_cmd.go:164-172` (`promptYN` `[Y/n]`, EOF→yes)

- [ ] **Step 1 — decide one contract.** Pick a single default convention and a single non-TTY rule. The dangerous one is `promptYN` treating EOF as "yes" (init from closed stdin silently auto-accepts every dir-creation prompt) — change EOF to a safe default (no/abort) or require explicit `--yes`. Document the chosen convention in `prompts.go`.
- [ ] **Step 2 — test.** Add a test that piping empty/closed stdin to the affected prompts does NOT silently accept. Run.
- [ ] **Step 3 — commit.** `fix(cmd): consistent prompt defaults + safe non-TTY behavior (AUDIT-035)`.

### Task 5.5: Extract read-command preamble helper (AUDIT-016, Medium; cross-ref -030)

**Files:** New `func (cfg *config) walkOrdered() ([]RawNode, error)` (or similar); replace the copied `resolveEnv → Walk → filterWithPrompt → Order` blocks in `dag_cmd.go:24-39`, `list_cmd.go:47-65`, `bundle.go:44-62`, `compose_cmd.go:33-42,62-74`, `package.go:31-39,73-81,102-110`

- [ ] **Step 1 — extract.** Create the helper (mirror existing write-path `runPipeline`, main.go:283-305). Replace the 6+ duplicated blocks; this also fixes the already-drifted error strings (`"walk: %w"` vs `"walk %s: %w"`). Decide whether the helper runs `ValidateNodes` — see Task 6.4.
- [ ] **Step 2 — test + run.** Existing command tests must stay green. Commit: `refactor(cmd): extract walkOrdered read-path helper (AUDIT-016)`.

---

## W6 — API surface, dead code, and design consistency (AUDIT-014, -015, -020, -021, -024, -025, -026, -027, -028, -029, -030, -031, -032)

### Task 6.1: Wire or remove the predicate function registry (AUDIT-026, High; cross-ref -045, -052)

**Files:** `internal/predicate/eval.go:85-94` (`NewEvaluator`), `internal/packages/packages.go:1-2` (doc), `packages.Installed`/`Installable`

- [ ] **Step 1 — decide.** Either (a) register `installed()`/`installable()` so the documented predicates work — wire `packages.Installed/Installable` into the registry `NewEvaluator` builds, OR (b) if not wanted, remove the false package doc and the unused `Register`/`Warn`/`Mode` machinery. **Recommend (a)** — the doc advertises it and the functions exist.
- [ ] **Step 2 — test.** If (a): add an eval test that `@when installed(sh)` resolves (no "unknown function" error). If (b): assert the doc no longer claims it. Run.
- [ ] **Step 3 — commit.** `fix(predicate): wire installed()/installable() into evaluator (AUDIT-026)`.

### Task 6.2: Resolve the dead `setup.Scaffold` API (AUDIT-027, High; cross-ref -028, -040)

**Files:** `internal/setup/setup.go:15-59` + `internal/setup/shell.go:46-96`; live consumer `cmd/dotd/teardown_cmd.go`; live scaffolder `cmd/dotd/setup_cmd.go`/`init_cmd.go`

- [ ] **Step 1 — pick one scaffolding implementation.** Two scaffold paths exist (the unused `setup.Scaffold` library vs the inline code in `dotd setup`/`init`). Decide which is canonical. **Recommend:** make the commands call `setup.Scaffold` (it's tested) and delete the inline duplication — OR delete `setup.Scaffold`+`Options`+`Result`+`Action`+consts and their tests if the inline path is preferred. Whichever you keep must be the one with tests.
- [ ] **Step 2 — handle the writer half (AUDIT-028/040).** `AppendSourceLine`/`SourceLine` (shell.go) are the orphaned RC-writer; either wire them as the thing that installs the source line (so teardown's `RemoveSourceLine` has a matching writer) or remove them. Add a round-trip test pairing append+remove (closes AUDIT-040's missing round-trip).
- [ ] **Step 3 — run & commit.** `go test ./internal/setup/... ./cmd/...`. Commit: `refactor(setup): single scaffold path, wire/remove source-line writer (AUDIT-027, AUDIT-028)`.

### Task 6.3: Remove remaining dead exports (AUDIT-031, Low)

**Files:** `internal/packages/catalog.go:106-107` (`DetectInstalled`), `internal/predicate/ast.go:76` (`And`), `internal/packages/packages.go:74-98` (`PackageEntry.Check`)

- [ ] **Step 1 — `.Check` first (user-visible).** `PackageEntry.Check` is assigned from yaml but never read — either implement it (use the custom shell expr in `Installed`) or remove the field + document that `check:` is unsupported. **Recommend implement**, since it's a documented user knob. Then remove the truly-inert `DetectInstalled` and `predicate.And` if still unused (recheck after Task 6.1, which may consume them). Run tests. Commit: `refactor: remove/implement dead exports (AUDIT-031)`.

### Task 6.4: Apply `ValidateNodes` consistently across read commands (AUDIT-029, Medium)

**Files:** `cmd/dotd/main.go:296`; the read commands listed in the finding (resolve together with Task 5.5)

- [ ] **Step 1 — centralize.** Add the `ValidateNodes` call into the `walkOrdered` helper from Task 5.5 so `list`/`dag`/`bundle`/`compose`/`package` validate the same way `apply`/`check` do. (Decide: error vs warn for read-only commands — recommend error for consistency, since a config that `apply` rejects shouldn't look valid under `list`.)
- [ ] **Step 2 — test.** Add a test that a sequencing-conflict config fails under `dotd list`/`dag check`, not just `apply`. Run. Commit: `fix(cmd): validate nodes uniformly across commands (AUDIT-029)`.

### Task 6.5: Consolidate conflict detection (AUDIT-014, Medium; + AUDIT-015 dup blocks)

**Files:** `internal/pipeline/actions.go:75-80` (`validateNode`, unresolved dests), `internal/pipeline/act.go:108-145,125-159` (cross-node, resolved + the two byte-identical blocks)

- [ ] **Step 1 — safety net.** Ensure Task 2.7 (mergeActions handoff) and the W2 Act tests are green.
- [ ] **Step 2 — normalize before compare.** Make `validateNode` compare resolved/normalized dests (expand `~`) so it agrees with `Act`'s cross-node check — closes the "passes check, fails apply" gap. Extract the duplicated nosource-scan + link-conflict blocks in `Act` (AUDIT-015) into one helper called by both the compose and regular branches.
- [ ] **Step 3 — test.** Add a test where `~/.x` and `/home/u/.x` are detected as the same conflict at validate time. Run. Commit: `fix(pipeline): unify conflict detection + dedup Act blocks (AUDIT-014, AUDIT-015)`.

### Task 6.6: Factory for single-file `RawNode` (AUDIT-024, Medium)

**Files:** `internal/adopter/adopter.go:109-113`, `internal/pipeline/walk.go:41-53`

- [ ] **Step 1 — add constructor.** Add an exported `pipeline.NewFileNode(path, logicalName string, actions []Action) RawNode` that owns which fields `Act` requires; have adopter call it instead of hand-building the struct. Run adopter + pipeline tests. Commit: `refactor(pipeline): factory for single-file RawNode (AUDIT-024)`.

### Task 6.7: Move `fileExists` to a neutral home (AUDIT-025, Low)

- [ ] Move `fileExists` from `cmd/dotd/teardown_cmd.go:148` into `internal/fileutil` (or `prompts.go`-adjacent util); update `unapply_cmd.go:105` and teardown. Commit: `refactor: relocate generic fileExists helper (AUDIT-025)`.

### Task 6.8: Tilde-expansion dedup + inline `composeGenName` + `mergeActions` precondition (AUDIT-020, -021, -030, Low)

- [ ] **AUDIT-021:** inline `pipeline.ComposeFileName(n.Path)` at `cmd/dotd/compose_cmd.go:49`; delete the `composeGenName` wrapper.
- [ ] **AUDIT-020:** factor the `~`/`~/` branch shared by `init_cmd.go:174-182` (`expandTildeStr`) and `act.go:226-243` (`expandDest`) into a shared helper (leaf package) so future tilde-rule changes stay in sync.
- [ ] **AUDIT-030:** the `walkOrdered` helper (Task 5.5) plus the unified parsing already reduce the `mergeActions` precondition risk; add a one-line doc + a test that `mergeActions` on un-normalized input is never reached (or make it defensively normalize). Commit: `refactor(pipeline): inline composeGenName, share tilde expansion (AUDIT-020, AUDIT-021, AUDIT-030)`.

### Task 6.9: Consistent error-message prefixing in `cmd/dotd` (AUDIT-032, Low)

- [ ] Adopt one convention (recommend `command:` prefix for user-facing command errors, keeping `internal/*` stage prefixes for wrapped lib errors). Normalize the bare `walk:`/`order:`/`act:` prefixes in `main.go` and `dag_cmd.go` to carry a command qualifier. Run tests. Commit: `style(cmd): consistent error prefixes (AUDIT-032)`.

---

## W7 — UX / CLI streams and help (AUDIT-033, -034, -036)

### Task 7.1: `unapply` partial-failure exit code + stream (AUDIT-033, High; cross-ref -034)

**Files:** `cmd/dotd/unapply_cmd.go:46,136-149,151`

- [ ] **Step 1 — fix both defects.** Route the `os.Remove`-failure messages to stderr (`cmd.ErrOrStderr()` / `cfg.log`), not `cmd.OutOrStdout()`; accumulate failures and `return` a non-nil error so the command exits non-zero on partial failure.
- [ ] **Step 2 — test.** Add a test where one remove fails (e.g. permission); assert non-zero exit and error on stderr. Run.
- [ ] **Step 3 — commit.** `fix(cmd): unapply exits non-zero on partial failure, errors to stderr (AUDIT-033)`.

### Task 7.2: Route diagnostics to stderr across commands (AUDIT-034, Medium)

**Files:** `cmd/dotd/teardown_cmd.go:48,54`, `unapply_cmd.go:138,145`, `compose_cmd.go:94,102`

- [ ] **Step 1 — fix.** Send `ui.Warnf`/`ui.Errf`/`Missingf`/`Wrongf` diagnostic output to stderr per the `internal/log/log.go:2-3` contract; keep data/preview on stdout. Make `compose check` put both markers and summary on the same (stderr) stream.
- [ ] **Step 2 — test + commit.** Add/adjust tests asserting diagnostics aren't in captured stdout. Commit: `fix(cmd): diagnostics to stderr per log contract (AUDIT-034)`.

### Task 7.3: Fix `config edit` help text (AUDIT-036, Low)

- [ ] Change `cmd/dotd/config_cmd.go:90` `Short` from "Open dotcfg.yaml..." to "Open config.yaml in $EDITOR" (matches the file actually opened at `:96`). Commit: `docs(cmd): correct config edit help filename (AUDIT-036)`.

---

## W8 — Test backfill: CLI commands + remaining internals (AUDIT-039, -040, -041, -043, -044, -045, -046, -051, -052, -053, -056, -057, -058, -059, -061, -063, -064, -065, -066, -067, -068)

Each task: add the missing test(s), confirm coverage moved, commit. Group commits by area. (AUDIT-040's source-line round-trip may already be done in Task 6.2 — skip if so.)

### Task 8.1: `internal/fileutil` atomic-write tests (AUDIT-039, High)
- [ ] Create `internal/fileutil/fileutil_test.go`. Test `SaveYAML`: successful round-trip + indentation (SetIndent 2), parent `MkdirAll`, and the temp-file cleanup on encode/rename failure (inject an unwritable dir). Commit: `test(fileutil): cover SaveYAML atomicity + cleanup (AUDIT-039)`.

### Task 8.2: `internal/setup/shell.go` source-line + detect tests (AUDIT-040, High)
- [ ] Cover `DetectShellConfig` (per shell/OS → correct RC path), and a round-trip `AppendSourceLine`→`HasSourceLine`→`RemoveSourceLine` (closes the header-drift risk). Commit: `test(setup): cover shell RC detection + source-line round-trip (AUDIT-040)`.

### Task 8.3: TTY missing-key prompt path (AUDIT-041, High; cross-ref -022)
- [ ] Refactor `promptMissingKeys` (filter_prompt.go:54) to accept an injectable input source, then test the TTY+missing-keys path: prompt → augment env → re-filter includes the node. Cover `printPersistHint`. Commit: `test(cmd): cover interactive missing-key prompt (AUDIT-041)`.

### Task 8.4: `dotd adopt` command path (AUDIT-043, High; cross-ref -018)
- [ ] Test `runAdopt` non-interactive (`--force`/non-TTY): convention loading from a real `.dagger`, destination inference, `--to` handling, invoking `adopter.Adopt`. Commit: `test(cmd): cover adopt command path (AUDIT-043)`.

### Task 8.5: `bundle` transitive deps + quoting (AUDIT-044, High)
- [ ] Add a fixture with `@after` chains; assert `collectDeps` emits transitive deps in DAG order and `shellQuote`/`--include-env` work. Commit: `test(cmd): cover bundle transitive deps + env quoting (AUDIT-044)`.

### Task 8.6: predicate `Keys`/`And`/OR-collection (AUDIT-045 + -053, High/Medium)
- [ ] In `predicate_test.go` + `filter_test.go`: test every `Keys()` method, `And()`'s three branches (empty→True, single→passthrough, many→AndExpr), `collectKeys` dedup, and the OR caveat (keys collected from all OR branches even when one is satisfied). Commit: `test(predicate): cover Keys/And/OR key collection (AUDIT-045, AUDIT-053)`.

### Task 8.7: ecosystem default paths + ResolvePath contracts (AUDIT-046 + -064, High/Low)
- [ ] Test `DefaultBinDir` (NOT XDG), `DefaultGeneratedDir`, `DefaultConfigFile`, `XdgConfigHome`, the home-unavailable error branches; and in `ResolvePath` test that the env var is read via the *passed* name (use distinct names per case) + that tilde is intentionally not expanded. Commit: `test(ecosystem): cover default paths + ResolvePath contracts (AUDIT-046, AUDIT-064)`.

### Task 8.8: Order self/unknown-ref branches (AUDIT-051, Medium)
- [ ] Test: `@after` referencing a nonexistent name is ignored; a node whose `@after` prefix matches itself creates no self-edge (e.g. node `a` with `After:["a/"]`); empty `ResolveAfterRef` match set. Commit: `test(pipeline): cover Order self/unknown-ref handling (AUDIT-051)`.

### Task 8.9: predicate constructor + nil-registry path (AUDIT-052, Medium; cross-ref -026)
- [ ] Test `NewEvaluator` directly (not struct literals); cover `evalCall` with `Funcs==nil` and a non-`exists` call. Commit: `test(predicate): cover NewEvaluator + nil-registry path (AUDIT-052)`.

### Task 8.10: `dotd init` scaffolding (AUDIT-056, Medium)
- [ ] Test `scaffoldDagger`/`scaffoldDaggerInteractive`: dir/file creation + skip-existing idempotency (mirror setup's idempotency test). Commit: `test(cmd): cover init scaffolding + idempotency (AUDIT-056)`.

### Task 8.11: `config`/`env edit` + `loadConfig` (AUDIT-057, Medium)
- [ ] Add `dotd config` show/get/set tests (none exist); cover `loadConfig` (reads `cfg.configPath`). For edit cmds, inject a fake `$EDITOR` (e.g. `true`/a script) and assert the correct file path is passed. Commit: `test(cmd): cover config commands + edit path (AUDIT-057)`.

### Task 8.12: `get-os`/`get-hostname` detectors (AUDIT-058, Medium; cross-ref -068)
- [ ] Test the hidden `get-os` command incl. darwin→macos normalization, and `get-hostname`. Commit: `test(cmd): cover get-os/get-hostname canonical detectors (AUDIT-058)`.

### Task 8.13: `unapply --all` path + flag shadow (AUDIT-059, Medium)
- [ ] Test `--all`: ignores `@when`, removes every symlink pointing into `cfg.files`, and the local-flag-shadows-root-flag binding. Commit: `test(cmd): cover unapply --all and flag shadow (AUDIT-059)`.

### Task 8.14: `dotd check` actually detects drift (AUDIT-061, Medium)
- [ ] Add negative tests: missing symlink, wrong-target symlink, stale init.sh → assert non-zero exit and a `Missing`/`Wrong` report (today tests only assert presence of "symlinks:"). Commit: `test(cmd): assert check detects drift, not just summary (AUDIT-061)`.

### Task 8.15: predicate Register panic + MissingKeyError (AUDIT-063, Low)
- [ ] Assert `Register` panics on duplicate; assert `MissingKeyError.Error()` message + type identity (`errors.As`). Commit: `test(predicate): cover Register panic + MissingKeyError (AUDIT-063)`.

### Task 8.16: env `Resolve` 3-layer + no-mutation (AUDIT-065, Low)
- [ ] Test all three layers overriding the same key (cli wins) and that `Resolve` does not mutate input maps. Commit: `test(env): cover triple-override + input immutability (AUDIT-065)`.

### Task 8.17: config/env Save errors + KnownFields strictness (AUDIT-066, Low)
- [ ] Test `Save` error wrap (unwritable dir); test `config.loadFrom` rejects an unknown field (`KnownFields(true)`); test `env.load` malformed-YAML error. Commit: `test(config,env): cover Save errors + KnownFields strictness (AUDIT-066)`.

### Task 8.18: `internal/ui` + `internal/log` tests (AUDIT-067, Low)
- [ ] Create test files: assert `log.New` parses each level in `LevelNames` and rejects bad names (the `--log-level` validation at main.go:70); smoke-test `ui` status helpers' output + stream. Commit: `test(ui,log): add baseline coverage (AUDIT-067)`.

### Task 8.19: Real env-resolution chain test (AUDIT-068, Low; cross-ref -058)
- [ ] Add one apply/list test that feeds an env.yaml containing `os: $(echo linux)` (or `$(dotd get-os)`) and proves shell-expansion → normalize works end-to-end, rather than only `--env os=linux` overrides. Commit: `test(cmd): cover real env.yaml expansion path (AUDIT-068)`.

---

## W9 — e2e / CI hardening (AUDIT-060, -062)

### Task 9.1: Remove self-skipping conflict assertion (AUDIT-060, Medium; cross-ref -062)

**Files:** `cmd/dotd/integration_test.go:341-349`

- [ ] **Step 1 — remove escape hatch.** Delete the branch that logs "apply without --force unexpectedly succeeded; skipping conflict test" and `return`s. Make the test hard-assert that apply without `--force` over a plain file errors and does NOT create the symlink. Run; if it now fails, conflict protection is already broken — fix it (and see Task 6.5). Commit: `test(cmd): conflict test hard-fails instead of self-skipping (AUDIT-060)`.

### Task 9.2: Run e2e against current source, not the released binary (AUDIT-062, Medium)

**Files:** `test/run-e2e.sh:7-13`, `test/e2e/procure/release.sh:8`; existing `test/run-e2e-local.sh` builds from source

- [ ] **Step 1 — make source-build the default gate.** Promote the source-build path (`run-e2e-local.sh` / `procure/local.sh`) to the canonical pre-merge e2e, so the suite validates HEAD. Keep the release-install variant as a separate post-release smoke test if desired. Wire the source-build e2e into CI (or document how it runs) so conflict/idempotency/dry-run regressions are caught before merge, not after a release ships.
- [ ] **Step 2 — commit.** `test(e2e): default suite builds from source to gate HEAD (AUDIT-062)`.

---

## Self-Review (completed during authoring)

**Spec coverage:** All 68 AUDIT findings are assigned to exactly one task. Mapping by ID: W1{001-003}, W2{037,038,042,047,048,049,050,054,055}, W3{006,013,017,019}, W4{004,005,007,008,009,010,011,012}, W5{016,018,022,023,035}, W6{014,015,020,021,024,025,026,027,028,029,030,031,032}, W7{033,034,036}, W8{039,040,041,043,044,045,046,051,052,053,056,057,058,059,061,063,064,065,066,067,068}, W9{060,062}. Count = 68. ✓

**Cross-references honored:** 013↔006/017 (W3 ordered after W2 safety net), 022↔018/053, 014↔015/055, 026↔045/052, 027↔028/040, 029↔016/030, 037↔013/042, 060↔062.

**Sequencing rationale:** W1 (correctness) → W2 (safety net over untested code) → W3 (refactors that depend on W2 tests) → W4 (mechanical constants) → W5/W6 (structural) → W7 (UX) → W8 (test backfill) → W9 (CI). W3.1, W6.5 explicitly gate on W2 tests being green.

**Deviation from default plan format (intentional):** This is a 68-item remediation backlog, not a single greenfield feature. Code-fix tasks specify the exact change + verification rather than full pre-written implementations, because the fixes are edits to existing code best written against the live file (and pre-writing 68 implementations would be speculative). Test-backfill tasks specify the behavior + assertion to add. Every task is concrete: exact files, exact change, exact verify command.
