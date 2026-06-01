# Test Quality

Coverage and test-design gaps: untested production logic, stale skip directives, self-skipping assertions, and tests that bypass real resolution paths. Per-function figures from `go test -cover` / `go tool cover`.

### [AUDIT-037] Entire composition feature untested end-to-end; compose tests skipped with false "not implemented" reasons

**Original ID:** T-053 (merged: T-001, T-005)
**Location:** `internal/pipeline/act.go:41` (`Act` 42.9%) & `:90-137` (compose branch), `internal/pipeline/walk.go:305` (`parseDaggerActions` 0.0%) & `:116-160` (compose-target node emission), `cmd/dotd/compose_cmd.go:117` (`hasComposeAction` 0.0%) & `:126` (`composeGenName` 0.0%), `cmd/dotd/integration_test.go:452,472,495,509,517` (five `t.Skip`)
**Severity:** Critical
**Description:** Compose IS implemented in production (`Act` handles `ActionCompose`, `compose_cmd.go` is fully wired with staleness diff) yet ships with no automated coverage. The compose branch of `Act` (fragment collection, assembly, generated-file path derivation, synthetic source node, link-on-generated) is unexercised; `parseDaggerActions` (dir-level `.dagger` actions) is 0.0% with no composition-enabled fixture in `walk_test.go`; and all five compose integration tests (`TestComposeApply`, `TestComposePredicateGating`, `TestComposeList`, `TestComposeCheck_AfterApply`, `TestComposeCheck_Stale`) are `t.Skip`-ped with stale/false "not yet implemented in v2" reasons. There is no compose e2e script either.
**Justification:** The skip reasons are demonstrably false — the feature is live production code. This merges the pipeline-angle gaps (T-001 Act compose path, T-005 `parseDaggerActions`) with the command/e2e-angle gap (T-053 stale skips, `hasComposeAction`/`composeGenName` 0%). The single largest behavioral gap in scope.
**Impact:** Fragment mis-ordering, wrong generated filename, including inactive fragments, dropping the synthetic source node, failing to symlink the generated file, or `compose check` never reporting staleness would all ship green.
**Cross-reference:** AUDIT-013.

### [AUDIT-038] `deriveLinkDest` (empty-dest link derivation from link_root) untested

**Original ID:** T-002
**Location:** `internal/pipeline/act.go:206` (`deriveLinkDest`, 0.0%)
**Severity:** Critical
**Description:** When a link action has an empty `Dest`, the destination is derived from `LinkRoot` + `LinkRootDir` with `nosync-` stripping and `dot-` → `.` rewriting per path component (act.go:215-223). No test calls `Act` with an empty-`Dest` link node — `act_test.go` always passes an explicit `Dest` — so this transformation is entirely unverified. `validateNode` explicitly permits empty dest when `LinkRoot != ""`, so the path is reachable in production.
**Justification:** The `dot-` → `.` and `nosync-` rules are the core of how dotfiles map to `$HOME` targets; 0.0% coverage.
**Impact:** A wrong rewrite (failing to strip `nosync-`, mis-joining components) silently links files to the wrong home-relative path — a data-corruption-class bug for the user's home dir.

### [AUDIT-039] `internal/fileutil` has zero tests; atomic-write helper unverified

**Original ID:** T-050
**Location:** `internal/fileutil/fileutil.go:14` (`SaveYAML`)
**Severity:** High
**Description:** `SaveYAML` (the only symbol in the package) has no test file. It is the atomic write path (temp file → encode → rename) used to persist config.yaml/env.yaml/packages.yaml, with four distinct error/cleanup branches (`os.Remove(tmpPath)` on encode/close/rename failure) plus parent `MkdirAll` and `SetIndent(2)`. It is exercised only indirectly via `env set`/`config set` tests; no test asserts atomicity or the indentation contract.
**Justification:** No `_test.go` in the package; none of the cleanup/rename/indentation branches is directly exercised.
**Impact:** A regression dropping the rename (non-atomic write), leaking temp files, or changing YAML indentation would ship undetected.

### [AUDIT-040] `internal/setup/shell.go` source-line writers 0% covered; `AppendSourceLine` dead in prod and untested

**Original ID:** T-051
**Location:** `internal/setup/shell.go:25` (`DetectShellConfig` 0%), `:49` (`SourceLine` 0%), `:59` (`HasSourceLine` 0%), `:84` (`AppendSourceLine` 0%); only `RemoveSourceLine` (94.7%) is tested
**Severity:** High
**Description:** Four shell-RC functions have no tests. `AppendSourceLine` — which writes the `# dotd — generated shell init` header + source line into `.bashrc`/`.zshrc` — has no production caller AND no test. `RemoveSourceLine`'s test hardcodes the exact header `AppendSourceLine` would produce, so if `AppendSourceLine` drifted from that format, removal would silently stop matching, with no round-trip test pairing the two.
**Justification:** `go tool cover` reports the four at 0.0%; `AppendSourceLine` is also dead in production (no caller).
**Impact:** Shell-RC wiring (what makes init.sh actually load) could break — wrong RC path per shell/OS, wrong source line, or header mismatch breaking later removal — with green tests throughout.
**Cross-reference:** AUDIT-028.

### [AUDIT-041] TTY missing-key prompt feature (`promptMissingKeys`) is 0% covered — the headline feature of the last 4 commits

**Original ID:** T-058
**Location:** `cmd/dotd/filter_prompt.go:54` (`promptMissingKeys` 0%), `:78` (`printPersistHint` 0%), `:20` (`filterWithPrompt` 40.0%); test `filter_prompt_test.go:48`
**Severity:** High
**Description:** `filterWithPrompt` is tested only on non-TTY paths and TTY-with-no-missing-keys. The actual interactive path — TTY + missing keys → huh prompt → augment env → re-filter — has zero coverage; `promptMissingKeys` and `printPersistHint` are never run. This feature is brand-new (recent commits `f85430c`, `8d24ba9`), and the branch it was written for is exactly the untested one. The `os.Exit(1)` on `huh.ErrUserAborted` is also unreachable by any test.
**Justification:** `promptMissingKeys 0.0%`, `printPersistHint 0.0%`, `filterWithPrompt 40.0%`. `promptMissingKeys` could be refactored to take an injectable input source but currently is wholly unverified.
**Impact:** The prompt could collect the wrong keys, fail to augment the env (re-filter still excludes nodes), or the abort-exit could regress, all while the existing tests stay green.
**Cross-reference:** AUDIT-022.

### [AUDIT-042] `package check` / `package generate` integration tests skipped; only `package list` exercised

**Original ID:** T-054
**Location:** `cmd/dotd/integration_test.go:401,420` (`t.Skip`), `cmd/dotd/package.go:133` (`loadRegistry` 0.0%)
**Severity:** High
**Description:** `TestPackageRequireHardFail` (`package generate`) and `TestPackageCheckOutput` (`package check`) are `t.Skip`-ped ("not yet migrated to v2"), so those commands have no command-level test; `loadRegistry` is 0.0%. The underlying `internal/packages` is well-tested, but the CLI glue — `loadRegistry` reading `<files>/packages.yaml`, `collectPackageRequests` flattening node `Require`/`Request` and skipping `IsCompose` nodes, passing `exec.LookPath` — is untested end to end. The skip reason is plausibly stale, like compose.
**Justification:** `loadRegistry 0.0%`; the `@require` hard-fail contract is unit-tested in `packages` but never verified through the actual command.
**Impact:** `package check`/`generate` could fail to load the registry, mis-map annotations to package requests, or wire the wrong lookup function, with no failing test.
**Cross-reference:** AUDIT-037.

### [AUDIT-043] `dotd adopt` command path is 0% covered (only `resolveToFlag` tested)

**Original ID:** T-055
**Location:** `cmd/dotd/adopt.go:57` (`runAdopt` 0%), `:146` (`conventionsFrom` 0%), `:160` (`promptAdoptConfirm` 0%)
**Severity:** High
**Description:** `runAdopt`, `conventionsFrom`, and `promptAdoptConfirm` have no test; only the pure helper `resolveToFlag` is tested. The `internal/adopter` library is well-tested (80%), but the command that loads `.dagger` conventions, infers the destination, prompts, and invokes `adopter.Adopt` is never run. The custom-convention path and inline TTY check are uncovered. The non-interactive `--force`/non-TTY path is testable with the existing harness but isn't.
**Justification:** `runAdopt 0.0%`; library unit tests give false confidence about the command glue.
**Impact:** `dotd adopt` could pass the wrong conventions, infer wrong destinations from a real `.dagger`, or mishandle `--to`, while green `adopter` unit tests mask it.
**Cross-reference:** AUDIT-018.

### [AUDIT-044] `bundle` transitive-dependency collection (`collectDeps`) only 8.7% covered

**Original ID:** T-056
**Location:** `cmd/dotd/bundle.go:135` (`collectDeps` 8.7%), `:175` (`shellQuote` 0.0%), `runBundle` 65.3%; test `main_test.go:407`
**Severity:** High
**Description:** `TestBundleSimple` bundles a single file with no `@after` dependencies, so the entire point of bundle — concatenating transitive `@after` deps in DAG order — is never exercised. `collectDeps` (BFS over `@after` edges via `pipeline.ResolveAfterRef`) is 8.7%; `shellQuote` (used by `--include-env`) is 0.0%.
**Justification:** The fixture has only `base.sh` with no `@after`; bundle's core contract and the `--include-env`/`--output` machinery are untested.
**Impact:** Bundle could emit deps in wrong order, miss transitive deps, fail to expand prefix `@after` refs, or shell-misquote env exports, and `TestBundleSimple` would still pass.

### [AUDIT-045] predicate: AST `Keys()` methods, `And()` constructor, and OR-branch key collection untested

**Original ID:** T-009 (merged: T-010)
**Location:** `internal/predicate/ast.go:29,39,51,62,70` (`Keys` methods, 0%), `:76` (`And`, 0%), `:87` (`collectKeys`, 0%), `:29` (`OrExpr.Keys`); `internal/pipeline/filter.go:41-43` (documented OR behavior); test `filter_test.go` has `AndBothMissing` only
**Severity:** High
**Description:** `predicate_test.go` never calls `.Keys()` on any Expr, nor `And()`/`collectKeys`. `And()` has three distinct branches (empty→TrueExpr, single→passthrough, many→AndExpr), none asserted. The documented OR caveat (`CollectMissingKeys` collects keys from all OR branches even when one is satisfied, filter.go:41-43) has no test — `filter_test.go` covers AND but not OR.
**Justification:** Every `Keys` method and `And`/`collectKeys` at 0.0% in the package. `And([])` returning the wrong identity or `collectKeys` failing to dedup would corrupt missing-key prompting. (T-010's OR gap is the same root cause and is merged here.)
**Impact:** Missing-key prompting (a TTY UX feature) and manifest conjunction logic could regress — e.g. OR short-circuiting would silently stop asking for keys on an unsatisfied branch — with no test guard.
**Cross-reference:** AUDIT-026.

### [AUDIT-046] ecosystem: XDG-data defaults, bin dir, generated dir, and config-file defaults untested

**Original ID:** T-013
**Location:** `internal/ecosystem/ecosystem.go:81` (`DefaultBinDir` 0%), `:91` (`DefaultGeneratedDir` 0%), `:52` (`DefaultConfigFile` 0%), `:25` (`XdgConfigHome` 0%); error branches of `xdgConfigHome`/`xdgDataHome` (33.3%)
**Severity:** High
**Description:** `ecosystem_test.go` tests `DefaultInitFile`/`DefaultEnvFile` but not `DefaultBinDir`, `DefaultGeneratedDir`, `DefaultConfigFile`, or exported `XdgConfigHome`. The home-dir-unavailable error branches are also uncovered. These are canonical-resolution endpoints, and the package is the lowest-covered in core scope at 47.7%.
**Justification:** All four defaults at 0.0%. The doc-comment says `DefaultBinDir` is deliberately NOT XDG — nothing tests that distinction.
**Impact:** A wrong default path (bin dir under XDG instead of `~/.local/bin`, or generated dir not under XDG_DATA) would silently relocate user files.

### [AUDIT-047] `~bin` / `~bin/` link-destination expansion untested

**Original ID:** T-003
**Location:** `internal/pipeline/act.go:227` (`expandDest`, 50.0%); branches at `:234-241`
**Severity:** High
**Description:** `act_test.go:80` covers `~/` expansion only. The `~bin` and `~bin/...` branches, which expand against `ActOptions.BinDir`, are never exercised. `BinDir` is a first-class resolved path; links into the user bin dir are a supported destination form.
**Justification:** `expandDest 50.0%`; the `~bin` branch is genuinely untested.
**Impact:** A regression in `~bin` handling (wrong join, or treating `~bin` as a literal path) would place executables in the wrong location undetected.

### [AUDIT-048] `Act` filesystem-write error paths and `createSymlink` conflict handling under-tested

**Original ID:** T-004
**Location:** `internal/pipeline/act.go:245` (`createSymlink`, 33.3%); write pass `:164-182`
**Severity:** High
**Description:** `createSymlink` handles three cases at an existing dest: existing symlink (remove+relink), non-symlink with `Force` (remove), non-symlink without force (error). Only the no-pre-existing-file happy path is tested. No test covers re-linking over an existing symlink, the `Force` overwrite of a real file, or the "exists and is not a symlink" error. Generated-file write errors are also untested.
**Justification:** `createSymlink 33.3%`; the `Force` flag and "refuse to clobber a real file" guard are safety-critical — they decide whether a user's real config file gets deleted.
**Impact:** A bug in the force/no-force decision could destroy a user's real dotfile without `--force`, or fail to overwrite when `--force` is given. Neither is caught.

### [AUDIT-049] Walk: `.dagger files:` dict entries (binary/JSON/Lua files) entirely untested

**Original ID:** T-006
**Location:** `internal/pipeline/walk.go:248-299` (the `files:` dict loop); `Walk` 46.0%
**Severity:** High
**Description:** The second walk pass processes explicit `files:` entries from `.dagger` — the only mechanism for managing files that can't carry annotations. It handles `disable`, `name`, `when`, per-file `actions`/`after`/`require`/`request`, dedup-by-type, and silent-skip of missing files. No test fixture defines a `.dagger` with a `files:` map, so none of this executes.
**Justification:** `Walk 46.0%`; no `walk_test.go` assertion sources a node from a `files:` dict (all are annotation-derived).
**Impact:** Dedup-by-type, disable handling, and name/when overrides for dict-declared files could all break silently — the only path for managing non-text dotfiles.

### [AUDIT-050] Order: tie-break correctness and `mergeSortedByName` under-tested

**Original ID:** T-007
**Location:** `internal/pipeline/order.go:112` (`mergeSortedByName`, 54.5%); alpha tie-break `:60,76-77`
**Severity:** Medium
**Description:** `order_test.go` asserts simple alpha ordering and a single `after` edge but never tests the deterministic tie-break among multiple nodes that become ready simultaneously after a dependency resolves — the exact scenario `mergeSortedByName` exists for. The both-non-empty interleave branch is only 54.5% covered.
**Justification:** `mergeSortedByName 54.5%`; it is the determinism guarantee for DAG output order, and only the trivial frontier path is tested.
**Impact:** A regression making output order non-deterministic (wrong comparison, dropped element on a tail slice) would change init.sh source order run-to-run with no test failing.

### [AUDIT-051] Order: `@after` prefix self-match, unknown-ref ignore, and self-reference skip untested

**Original ID:** T-008
**Location:** `internal/pipeline/order.go:42` (unknown-ref `continue`), `:44` (self-ref skip), `:90` (`ResolveAfterRef`)
**Severity:** Medium
**Description:** `TestOrder_PrefixAfter` covers the happy prefix case, but three documented behaviors are unasserted: an `@after` referencing a nonexistent logical name is silently ignored; a node whose `@after` prefix matches itself must not create a self-edge; `ResolveAfterRef` returning an empty match set. The prefix test's node does not match its own prefix, so the self-match guard is never hit. (Note: `ResolveAfterRef` itself is 100% via other callers — the gap is the self/unknown-ref branches in Order.)
**Justification:** Lines order.go:42 and :45 are reachable but no test drives them (e.g. node `a` with `After: ["a/"]`).
**Impact:** If the self-reference guard regressed, a node matching its own prefix would create a false cycle and `Order` would erroneously error on a valid config.

### [AUDIT-052] predicate: `NewEvaluator` constructor untested; `evalCall` no-registry error path untested

**Original ID:** T-011
**Location:** `internal/predicate/eval.go:89` (`NewEvaluator`, 0.0%), `:175` (no-registry error); `evalCall` 77.8%
**Severity:** Medium
**Description:** `NewEvaluator` (the canonical constructor wiring a Strict registry) shows 0.0% — all eval tests build `&Evaluator{...}` literals instead. The `evalCall` branch where `Funcs == nil` and the call is not `exists` is also untested (`TestEvalCustomFunc` always sets `Funcs`).
**Justification:** `NewEvaluator 0.0%`. The constructor is meant to ensure filter and manifest evaluation share identical capabilities, yet tests bypass it.
**Impact:** If `NewEvaluator` stopped registering built-ins or set the wrong mode, filter/manifest eval would silently diverge; the nil-registry error path is unverified.
**Cross-reference:** AUDIT-026.

### [AUDIT-053] predicate: `OrExpr` key collection across branches (the documented OR caveat) untested

**Original ID:** T-010
**Location:** `internal/predicate/ast.go:29` (`OrExpr.Keys`, 0%), `internal/pipeline/filter.go:41-43`
**Severity:** Medium
**Description:** `CollectMissingKeys` documents that OR collects keys from all branches even when one is satisfied. `filter_test.go` tests AND but has no OR test, so the documented, deliberately-surprising OR behavior is unverified. (This is the same root cause as AUDIT-045 viewed from the filter/pipeline angle; cross-referenced rather than double-counted.)
**Justification:** No `filter_test.go` test constructs an OR between distinct keys; `OrExpr.Keys` at 0% in the predicate package.
**Impact:** If OR key-collection regressed to short-circuit, the missing-key prompt would silently stop asking for keys on the unsatisfied branch.
**Cross-reference:** AUDIT-045.

### [AUDIT-054] Walk: `@disable` / `files: disable` and the `disabled` return slice untested

**Original ID:** T-017
**Location:** `internal/pipeline/walk.go:188-191` (`@disable`), `:256-259` (`files:` disable); `cascadeState` 76.2%, `combineWhen` 85.7%
**Severity:** Medium
**Description:** Every `walk_test.go` call discards the second return value (`nodes, _, err`). No test asserts that a `@disable`-annotated file is excluded from `nodes` AND appears in the `disabled` slice. `@disable` is the user's mechanism to opt a file out.
**Justification:** All seven `Walk` tests use `nodes, _, err`; the `disabled` slice is never inspected.
**Impact:** A regression that included disabled files (or failed to report them) would silently re-enable dotfiles the user explicitly turned off.

### [AUDIT-055] pipeline: `mergeActions` second-explicit-link "keep both for validateNode" branch untested

**Original ID:** T-018
**Location:** `internal/pipeline/walk.go:467-478`; `mergeActions` 76.1%
**Severity:** Medium
**Description:** `TestMergeActions_CanonicalAnnotations` covers single link, inherited-link override, and source+link, but not two explicit link annotations on the same file with different dests, which `mergeActions` deliberately keeps both (so `validateNode` can report the conflict). The `alreadyPresent` dedup is also unexercised.
**Justification:** `mergeActions 76.1%`; the two-explicit-links path is the handoff between merge and validation — a documented design point with no test.
**Impact:** If this branch regressed (silently dropping the second link), `validateNode`'s conflict detection would never fire and a real link conflict would ship silently.
**Cross-reference:** AUDIT-014.

### [AUDIT-056] `dotd init` scaffolding is 0% covered; only the precondition error is tested

**Original ID:** T-057
**Location:** `cmd/dotd/init_cmd.go:96` (`scaffoldDaggerInteractive` 0%), `:129` (`scaffoldDagger` 0%), `:164` (`promptYN` 0%)
**Severity:** Medium
**Description:** The only init test (`TestInit_RequiresConfig`) asserts init errors when config.yaml is absent — only the guard clause. The actual scaffolding (`scaffoldDagger`, `scaffoldDaggerInteractive`, `promptYN`) is never run, including the documented skip-existing idempotency claim and directory/file creation. By contrast `setup`'s scaffolding IS tested for idempotency.
**Justification:** All three at 0.0%; init's `.dagger`-writing path has no equivalent of setup's idempotency test.
**Impact:** `dotd init` could write malformed `.dagger` files, fail to create convention dirs, or break skip-existing idempotency, undetected.

### [AUDIT-057] `config`/`env edit` ($EDITOR) and `loadConfig` paths near-zero coverage

**Original ID:** T-059
**Location:** `cmd/dotd/config_cmd.go:26` (`loadConfig` 0%), `:87` (`newConfigEditCmd` 11.1%), `cmd/dotd/env.go:91` (`newEnvEditCmd` 10.0%)
**Severity:** Medium
**Description:** `loadConfig` is 0.0% — no `config show`/`get`/`set` command test exists (the `"config"` matches in `main_test.go` are dotfiles subdir paths, not commands). The `$EDITOR`-spawning `config edit`/`env edit` subcommands are effectively untested. There are dedicated `env show/get/set/diff` tests but no `config` command tests at all.
**Justification:** `loadConfig 0.0%`, edit cmds ~10%. `loadConfig` resolves and reads `cfg.configPath` — the canonical-path contract — and is unverified.
**Impact:** `dotd config` (show/get/set) could regress entirely undetected; edit commands could spawn `$EDITOR` with the wrong file path.

### [AUDIT-058] `get-os` / `get-hostname` (the canonical OS/host detectors) are untested

**Original ID:** T-060
**Location:** `cmd/dotd/getters.go:23` (`get-os`, darwin→macos normalization at `:27`), `:37` (`get-hostname`)
**Severity:** Medium
**Description:** The hidden `get-os`/`get-hostname` subcommands have no test. Per CLAUDE.md these are the canonical OS/host detectors (the only allowed `runtime.GOOS` site), wired into the env.yaml template (`os: $(dotd get-os)`). The darwin→macos normalization is behavioral: if it broke, every `@when(os=macos)` predicate in a real install would silently never match.
**Justification:** No test references either command; the normalization that all macOS predicate matching depends on is unverified.
**Impact:** A regression in OS normalization would break predicate matching for all macOS users with no failing test.
**Cross-reference:** AUDIT-068.

### [AUDIT-059] `unapply --all` path (ignores `@when`, removes every repo symlink) is untested

**Original ID:** T-061
**Location:** `cmd/dotd/unapply_cmd.go` (`--all` branch at `:62`); tests `main_test.go:774-925`
**Severity:** Medium
**Description:** All four unapply tests exercise only the default (filtered) path. The `--all` path — which walks all nodes with no predicate filter and removes any symlink pointing into `cfg.files`, plus the documented local-flag-shadows-root-flag workaround — has no test. The `--all` branch uses a different link-plan construction (`buildActOptions` + walk-all) than the default (`runPipeline`).
**Justification:** No `--all` literal in any unapply test; the cobra flag-collision shadow (local vs root persistent `--all`) is exactly the subtle wiring that needs a regression test.
**Impact:** `unapply --all` could remove the wrong set of symlinks (respect `@when` when it shouldn't, or remove links outside `cfg.files`), and the flag-shadow could silently bind to the wrong flag, undetected.

### [AUDIT-060] Integration conflict test can silently no-op (self-skipping assertion)

**Original ID:** T-062
**Location:** `cmd/dotd/integration_test.go:341-349`
**Severity:** Medium
**Description:** `TestSymlinkConflictWithForce` contains a branch that logs "apply without --force unexpectedly succeeded; skipping conflict test" and `return`s if apply did not error and the symlink got created. If conflict-detection regressed (apply overwrites a plain file without `--force`), the test would *pass* by taking the skip branch rather than failing. The test only ever asserts the `--force` success path; the no-force protection is conditionally abandoned. The e2e `conflict.sh` does this correctly (hard-fails), but the Go test has an escape hatch.
**Justification:** The quoted escape hatch masks the most important assertion; only the Docker e2e (not run in CI per the fetch-release design) would catch the regression.
**Impact:** A regression removing conflict protection (silently clobbering user files on apply) would not fail the Go integration suite.
**Cross-reference:** AUDIT-062.

### [AUDIT-061] `dotd check` tests assert presence of a summary, not actual conflict/drift detection

**Original ID:** T-063
**Location:** `cmd/dotd/main_test.go:189-225` (`main_test.go:196` asserts only `Contains(out, "symlinks:")`), `test/e2e/check.sh`
**Severity:** Medium
**Description:** `TestCheckEmptyRepo` only asserts the output contains `"symlinks:"` and `TestCheckAfterApply_Unit` only asserts check exits 0 after apply. Neither verifies check *detects* a problem — a missing symlink, a wrong-target symlink, or a stale init.sh. The e2e `check.sh` likewise only asserts check passes after a clean apply. There is no broken-state/negative test asserting a non-zero exit or a `Missing`/`Wrong` report.
**Justification:** `check` is the verification command; its entire reason for existing is detecting drift, and the negative case is untested.
**Impact:** `check` could stop detecting missing/wrong symlinks or stale init.sh entirely and still report "all good" — every test would pass, defeating the command's purpose.

### [AUDIT-062] e2e suite tests installed release binary, not current source; CI does not exercise it

**Original ID:** T-064
**Location:** `test/run-e2e.sh:7-13` (resolves `DOTD_VERSION` from latest GitHub release), `test/e2e/procure/release.sh:8` (installs published binary via `install.sh --version`)
**Severity:** Medium
**Description:** `run-e2e.sh` resolves `DOTD_VERSION` from the latest GitHub release and `procure/release.sh` installs that published binary, so the e2e suite validates an already-shipped artifact, not the working tree — always one release behind HEAD. A `run-e2e-local.sh`/source-build variant exists, but the canonical `run-e2e.sh` is release-based.
**Justification:** Combined with the missing compose e2e (AUDIT-037) and the self-skipping Go conflict test (AUDIT-060), the only hard conflict assertion (`conflict.sh`) runs against the previous release, not new code.
**Impact:** A conflict/idempotency/dry-run regression introduced in HEAD is invisible to `run-e2e.sh` until after a release ships it — the e2e is a post-hoc smoke test of releases, not a pre-merge gate.
**Cross-reference:** AUDIT-037, AUDIT-060.

### [AUDIT-063] predicate: `Register` duplicate-panic and `MissingKeyError.Error()` untested

**Original ID:** T-012
**Location:** `internal/predicate/eval.go:60` (`Register`, 66.7% — panic branch uncovered), `:17` (`MissingKeyError.Error`, 0.0%)
**Severity:** Low
**Description:** `Register` panics on duplicate registration; no test asserts this panic. `MissingKeyError.Error()` is 0.0% — `TestEval`'s "missing env key" case only checks `err != nil`, never the typed error or its message.
**Justification:** `Register 66.7%`, `MissingKeyError.Error 0.0%`. The missing-key error type is consumed elsewhere via `errors.As`-style handling; an unasserted message/type identity is fragile.
**Impact:** Duplicate-registration regressions (silent overwrite instead of panic) and a malformed error message ship undetected.

### [AUDIT-064] ecosystem: `ResolvePath` env-var-name and tilde-non-expansion contracts have no guard

**Original ID:** T-014
**Location:** `internal/ecosystem/ecosystem.go:112` (`ResolvePath`, 100% line coverage but behavior gaps)
**Severity:** Low
**Description:** The four precedence tests cover cli/shell/file/default in isolation (good), but there is no test that the env var is read via the *passed* `envVar` name — all four cases set `DOTD_TEST_VAR`, so a wrong-var-name regression would still pass — and no test documents that tilde is intentionally NOT expanded.
**Justification:** Line coverage 100% masks that the tests are structurally near-identical; the "tilde not expanded" contract has no guard. Documentation-as-test gap, surfaced per the borderline rule.
**Impact:** Low — precedence is exercised; mainly a documentation-as-test gap.

### [AUDIT-065] env: `Resolve` mutation-safety and full 3-layer single-key override not asserted

**Original ID:** T-015
**Location:** `internal/env/env.go:89` (`Resolve`, 100% line coverage)
**Severity:** Low
**Description:** `TestResolve_Precedence` checks cli>shell and shell>expanded, but no single test asserts all three layers overriding the *same* key simultaneously (cli wins), and no test asserts `Resolve` does not mutate its input maps. A behavior-thoroughness gap, not a line gap.
**Justification:** `Resolve 100%` line coverage; the all-three-on-one-key case is the strongest precedence assertion and is absent.
**Impact:** Low — individual precedences are covered; a subtle ordering bug only affecting triple-overlap is theoretically possible.

### [AUDIT-066] config/env: `Save` error paths and YAML decode-error (`KnownFields`) paths untested

**Original ID:** T-016
**Location:** `internal/config/config.go:54` (`Save`, 66.7%), `:43` (`loadFrom`, 83.3%); `internal/env/env.go:35` (`load`, 71.4%), `:146` (`Save`, 66.7%)
**Severity:** Low
**Description:** Both `Save` functions have only round-trip happy-path tests; the `fileutil.SaveYAML` error wrap (unwritable dir) is uncovered. `config.loadFrom` uses `dec.KnownFields(true)` — but no test verifies an unknown field in config.yaml produces a decode error, despite that being the field's explicit purpose. `env.load` malformed-YAML error is likewise untested.
**Justification:** `Save 66.7%` (both), `loadFrom 83.3%`. The `KnownFields(true)` strictness guardrail has zero coverage.
**Impact:** A regression dropping `KnownFields(true)` (silently accepting typo'd config keys) would not be caught; malformed-YAML error wrapping is unverified.

### [AUDIT-067] `internal/ui` and `internal/log` packages have zero tests

**Original ID:** T-052
**Location:** `internal/ui/ui.go:1`, `internal/log/log.go:1`
**Severity:** Low
**Description:** Neither package has a `_test.go`. `internal/ui` (status helpers) and `internal/log` (`New`, `LevelNames`) are entirely unverified. `log.LevelNames` is consumed by `main.go:70` to validate `--log-level` and `log.New` parses the level — a bad level-name map or parse would mis-handle CLI input.
**Justification:** No `_test.go` in either package; the `--log-level` validation logic is genuinely behavioral and untested in isolation.
**Impact:** Low — mostly cosmetic, but `--log-level` validation could regress undetected.

### [AUDIT-068] Apply/list/dag tests use `--env os=linux` to bypass the real env-resolution chain

**Original ID:** T-065
**Location:** `cmd/dotd/main_test.go:250-392` (e.g. `:272` `--env os=linux`), `cmd/dotd/integration_test.go:196`
**Severity:** Low
**Description:** Every apply/list/dag test injects env via `--env os=linux`/`--env context=...` overrides rather than resolving `os` through env.yaml's `$(dotd get-os)` expansion. Deterministic (good), but the production resolution path (shell-expand env.yaml → `get-os` → normalize) is never traversed; env.yaml fixtures are empty (`{}`). No test feeds an env.yaml containing `os: $(echo linux)` to prove expansion works end to end.
**Justification:** The override path is legitimate and well-tested, but the real-world env-resolution path is collectively untested across unit/integration/e2e. Surfaced per the borderline rule.
**Impact:** Low — the override path is legitimate; the shell-expansion + getters chain that a real user relies on is untested everywhere.
**Cross-reference:** AUDIT-058.
