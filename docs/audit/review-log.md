# Audit Review Log (Phase 4)

Every Phase-2/3 finding, independently re-verified against source, marked CONFIRMED or DISCARDED with a one-sentence justification. Borderline items are CONFIRMED with a **Note** stating the uncertainty.

**Tally:** 75 raw findings → 73 CONFIRMED, 2 DISCARDED (A-004 documented exception; F-007 refuted 'stub' lead — no defect).

| Dimension | Confirmed | Discarded |
|-----------|-----------|-----------|
| A Canonical | 3 | 1 (A-004) |
| B Magic values | 9 | 0 |
| C Duplication | 9 | 0 |
| D Ownership | 5 | 0 |
| E Design/API | 7 | 0 |
| F UX/CLI | 6 | 1 (F-007) |
| T Test quality | 34 | 0 |

---

# Review: Dimensions A & B

## Dimension A — Canonical Resolution

### A-001 — CONFIRMED
**Re-verified at:** `main.go:222` (no-op default fn → `cfg.linkRoot==""` in the common case), `main.go:258-261` (buildActOptions fills `os.UserHomeDir()`), `adopt.go:113` → `adopter.go:115` (raw `cfg.linkRoot` → `HomeDir`) → `act.go:42-44` (hard errors on empty `HomeDir`), `teardown_cmd.go:75` → `shell.go:28-34` (joins `home` with `.bashrc`/`.zshrc`).
**Justification:** The `$HOME` default for `linkRoot` is re-derived in three consumers and only `buildActOptions` applies it, so the same default-config user gets working `apply`, an `act: HomeDir is required` error on `adopt`, and cwd-relative RC paths on `teardown`.

### A-002 — CONFIRMED
**Re-verified at:** `main.go:201` (`cfg.configPath, err = dotcfg.DefaultPath()` — not routed through `ecosystem.ResolvePath`), `config.go:24-26` (`DefaultPath` → `ecosystem.DefaultConfigFile`); no `--config` flag exists in `main.go:60-67`.
**Justification:** Every other path field at `main.go:195-232` goes through `ResolvePath` with the flag/env/config/default chain, while `configPath` alone is set straight from the XDG default — a real structural asymmetry, no flag or `DOTD_CONFIG_FILE` tier.

### A-003 — CONFIRMED
**Re-verified at:** `setup_cmd.go:45` and `init_cmd.go:36` both call `dotcfg.DefaultPath()`; `main.go:73-74` runs `resolvePaths` in `PersistentPreRunE` which populates `cfg.configPath` (`main.go:34,201`); `config_cmd.go:35,55,75,82,96` correctly read `cfg.configPath`.
**Justification:** `cfg.configPath` is the canonical resolved value populated before any RunE and honored by the config subcommands, yet `setup`/`init` reach around it with a redundant `DefaultPath()`; init_cmd's "bootstrap" comment is inaccurate since `resolvePaths` has already loaded config before `runInit`.
**Note:** Borderline severity — harmless today because A-002 leaves `configPath` un-overridable, but becomes a latent divergence the moment A-002 is fixed; agent's Medium rating is fair.

### A-004 — DISCARDED
**Re-verified at:** `teardown_cmd.go:60` (`dotcfg.DefaultPath()`), `:64` (`env.DefaultPath()`), with the documented rationale at `:57-59` ("teardown removes the system-level files regardless of any --env-file / --files flag overrides").
**Justification:** This is a deliberately documented exception — teardown intentionally targets the installed system-level files rather than session overrides — so it is a sanctioned bootstrap/teardown exception, not a violation.
**Note:** The agent itself recorded this as LEGITIMATE; the only residual is the intra-command inconsistency (line 64 `env.DefaultPath()` vs `cfg.envFile` used by `resolveEnv` at line 72), which is intentional and worth a sanity confirm but not a defect.

## Dimension B — Magic Values

### B-001 — CONFIRMED
**Re-verified at:** `ecosystem.go:66` (canonical `filepath.Join(base, Name, "env.yaml")`), plus literal `env.yaml` in user-facing copy: `main.go` (4×), `env.go` (8×), `setup_cmd.go` (3×), `filter_prompt.go` (2×).
**Justification:** The functional path is centralized correctly, but ~15 hint/help strings spell `env.yaml` literally, so a filename rename in `ecosystem.go` leaves stale guidance pointing users at a nonexistent file.
**Note:** Borderline — copy-only divergence with no correctness/compile impact; agent's Medium rating is generous, Low would be defensible, but the surfacing is valid.

### B-002 — CONFIRMED
**Re-verified at:** `package.go:134` (reader: `filepath.Join(cfg.files, "packages.yaml")`) and `setup.go:78` (writer: `filepath.Join(opts.DotfilesDir, "packages.yaml")`) — two independent literals in different packages, no shared constant.
**Justification:** Reader and writer each spell `packages.yaml` literally with no `ecosystem.PackagesFile`/`packages.DefaultName`, so a one-sided rename silently desyncs scaffolding from the loader and installs become no-ops.

### B-003 — CONFIRMED
**Re-verified at:** `packages.go:239` (`strings.ReplaceAll(mgDef.Install, "{package}", pkgName)`) vs 24 catalog literals in `catalog.go:18-92+` (`dnf install -y {package}`, etc.).
**Justification:** The `{package}` token is the substitution contract defined nowhere as a constant; renaming it on one side leaves the literal placeholder in the executed shell command.
**Note:** The finding's prose at lines 77-78 is garbled ("only substitutes / Update is NOT substituted"), but the cited code and impact are correct.

### B-004 — CONFIRMED
**Re-verified at:** `walk.go:26-31` (`annAfter="after"` etc.), `dagger.go:17-23` (`yaml:"when"`, `yaml:"after"`, `yaml:"require"`, `yaml:"request"`, `yaml:"disable"`) and `:30` (`yaml:"name"`), `annotation.go:121` (bare `a.Key == "when"`).
**Justification:** The annotation vocabulary is independently declared as `ann*` constants, YAML struct tags, and a bare literal, so renaming e.g. `after` requires lockstep edits across three sites and a one-sided change silently drops `.dagger`-declared DAG edges.
**Note:** `annAction` has no `dagger.go` tag analog (`Actions` uses `yaml:"actions"`) and `When` has no `annWhen` constant — the triplication is exact for after/require/request/disable/name but slightly looser for action/when; the core risk stands.

### B-005 — CONFIRMED
**Re-verified at:** `packages.go:57` (bare `if key == "priority"`), `:89-91` (tags `yaml:"binary"`, `yaml:"check"`, `yaml:"prefer"`), `:102` (`known := map[string]bool{"binary":true,"check":true,"prefer":true}`).
**Justification:** The hand-written `UnmarshalYAML` filters known keys via bare literals that must match the struct tags above them, so renaming a tag without updating the `known` map causes a known field to be reprocessed as a package-manager entry.

### B-006 — CONFIRMED
**Re-verified at:** `adopter.go:23` (`ConventionNames{Shellrc:"shellrc", Bin:"bin", Config:"config"}`), `setup.go:63` (`[]string{"shellrc","config","bin"}`), `init_cmd.go:77,83,89` (`defDir:"shellrc"/"config"/"bin"`), `dagger.go:41-43` (`yaml:"shellrc"/"bin"/"config"`).
**Justification:** The three convention dir names are spelled as independent literals across four files in unrelated packages with no shared constant, so a one-sided rename scaffolds files where the pipeline never looks.

### B-007 — CONFIRMED
**Re-verified at:** `0o755`/`0o644` confirmed across `init_cmd.go:134,137`, `setup_cmd.go:144,148`, `act.go:170,173,246`, `initgen.go:41`, `adopter.go:169`, `fileutil.go:15`, `setup.go:95,105,108`, `shell.go:89,129`, `bundle.go:121` — ~16 production sites.
**Justification:** The mode literals are repeated widely; the agent correctly rates this Low since dir=755/file=644 is universal convention and no site is load-bearing against another (consistency, not correctness, risk).
**Note:** Borderline-Low — pure hygiene; surfaced only because a future hardening change would need to touch every site.

### B-008 — CONFIRMED
**Re-verified at:** `bundle.go:90` (`sb.WriteString("#!/bin/sh\n")`) and `packages.go:281` (`fmt.Fprintln(w, "#!/bin/sh")`).
**Justification:** Two independent generators emit the POSIX shebang as separate literals; agent's Low rating is correct since both are valid and self-contained, divergence is near-harmless.

### B-009 — CONFIRMED
**Re-verified at:** `env.go:125` (`strings.HasPrefix(e, "DOTD_")`) and `:128` (`rest := e[len("DOTD_"):]`) — the `"DOTD_"` literal twice on adjacent lines.
**Justification:** The two adjacent `"DOTD_"` occurrences are a classic change-one-miss-one hazard (mismatched prefix vs slice length silently mangles env keys); the `main.go` full-name vars are a separate scheme and correctly excluded from this finding.

---

## Review: Dimensions C & D

### C-001 — CONFIRMED
**Re-verified at:** `internal/pipeline/walk.go:305-324` (`parseDaggerActions`), `:406-419` (`parseActionString`), callsite `:131`
**Justification:** The two parsers genuinely diverge — `parseDaggerActions` only hardcodes compose/source/nosource/`link(...)` and silently drops anything else, while `parseActionString` handles arbitrary `type(dest)` with a malformed-paren fallback, so identical `actions:` strings behave differently at dir vs file level.

### C-002 — CONFIRMED
**Re-verified at:** `internal/annotation/annotation.go:77-91` (`parseKeyArgs`), `internal/pipeline/walk.go:406-419` (`parseActionString`)
**Justification:** `parseKeyArgs` and `parseActionString` are the byte-for-byte same `IndexByte('(')`/`LastIndexByte(')')`/TrimSpace/malformed-fallback algorithm differing only in return type; the finding correctly downgrades the third "copy" to C-001 since `parseDaggerActions` does not actually split.

### C-003 — CONFIRMED
**Re-verified at:** `cmd/dotd/adopt.go:92`, `cmd/dotd/filter_prompt.go:94-97` (`isTTYStdin`)
**Justification:** `adopt.go:92` inlines `term.IsTerminal(os.Stdin.Fd())` — the exact body of `isTTYStdin()` — while every other callsite routes through the helper; low severity but the same canonical-resolution anti-pattern. (**Note:** overlaps D-005; same single line.)

### C-004 — CONFIRMED
**Re-verified at:** `internal/pipeline/actions.go:75-80`, `internal/pipeline/act.go:127-130` & `:155-158`
**Justification:** validateNode compares unresolved `a.Dest` strings intra-node at validate time while `Act` compares fully `resolveLink`-ed paths cross-node at act time, so `~/.x` vs `/home/u/.x` passes validation but the "conflict" definition lives in two divergent places.

### C-005 — CONFIRMED
**Re-verified at:** `internal/pipeline/act.go:108-113` & `:140-145` (nosource), `:127-130` & `:155-158` (link-conflict)
**Justification:** Both nosource pre-scan loops are identical and both link-conflict blocks are identical (differing only in `Src`: `genPath` vs `n.Path`), duplicated across the compose and regular branches of the same `Act` function.

### C-006 — CONFIRMED
**Re-verified at:** `cmd/dotd/compose_cmd.go:117-124` (`hasComposeAction`), `internal/pipeline/act.go:82-88` (inline `hasCompose`), `internal/pipeline/actions.go:57-66` (`seenCompose`)
**Justification:** The "node has ActionCompose" predicate is implemented three times with no exported `pipeline` method to share; trivial and stable logic, but genuinely triplicated across two layers. (**Note:** Low severity — debatable whether worth a shared method.)

### C-007 — CONFIRMED
**Re-verified at:** `cmd/dotd/init_cmd.go:174-182` (`expandTildeStr`), `internal/pipeline/act.go:226-243` (`expandDest`)
**Justification:** The `~` and `~/` home-expansion branches are identical in both; they sit in different packages so cannot trivially share, but the tilde rule is now defined twice and could diverge if `~user`/empty-HOME handling is added. (**Note:** cross-package, so consolidation is non-trivial.)

### C-008 — CONFIRMED
**Re-verified at:** `cmd/dotd/compose_cmd.go:126-128`
**Justification:** `composeGenName` is a single-line pass-through to `pipeline.ComposeFileName(n.Path)` with one caller (`:49`), adding indirection but no abstraction value. (**Note:** trivial; pure inlining opportunity, no correctness impact.)

### C-009 — CONFIRMED
**Re-verified at:** `cmd/dotd/dag_cmd.go:24-39`, `list_cmd.go:47-65`, `bundle.go:44-62`
**Justification:** The resolveEnv→Walk→filterWithPrompt→Order preamble is line-for-line repeated, and the error wrap has already drifted (`"walk: %w"` in dag_cmd vs `"walk %s: %w"` in list/bundle) — concrete copy-paste divergence, a real missing `walkOrdered()` abstraction.

### D-001 — CONFIRMED
**Re-verified at:** `cmd/dotd/filter_prompt.go:38`
**Justification:** A filter-stage helper calls `os.Exit(1)` on `huh.ErrUserAborted`; grep confirms the only `os.Exit` sites are this and `main.go:28`, so the helper usurps main's exit ownership, bypasses cobra's error path, and is untestable through `Execute()`.

### D-002 — CONFIRMED
**Re-verified at:** `cmd/dotd/init_cmd.go:142,164,174` ↔ `cmd/dotd/setup_cmd.go:164,170`
**Justification:** init_cmd owns `promptDefault`/`promptYN`/`expandTildeStr` consumed by setup, while setup owns `printField`/`fieldPrompt` consumed by init (`init_cmd.go:100,111`) — genuine bidirectional coupling with no single home, made explicit by the existing `prompts.go` that holds only `promptConfirm`.

### D-003 — CONFIRMED
**Re-verified at:** `cmd/dotd/teardown_cmd.go:148` (`fileExists`), consumed at `cmd/dotd/unapply_cmd.go:105`
**Justification:** A fully generic stat helper lives in a specific command file yet is used by an unrelated sibling command, so unapply silently depends on teardown's presence; real cross-file coupling, though trivial. (**Note:** Low severity — borderline benign same-package sharing, surfaced because the owner file is arbitrary.)

### D-004 — CONFIRMED
**Re-verified at:** `internal/adopter/adopter.go:109-113`, `internal/pipeline/walk.go:41-53` (RawNode, 11 fields)
**Justification:** adopter hand-builds a `pipeline.RawNode` populating only 3 fields and feeds it to `Act`, encoding knowledge of which fields `Act` reads, and must import both `internal/node` (DeriveName) and `internal/pipeline` (struct) to assemble one node — no factory exists, so a future `Act` field dependency breaks adoption with no compile error.

### D-005 — CONFIRMED
**Re-verified at:** `cmd/dotd/adopt.go:92`, `cmd/dotd/filter_prompt.go:95-97` (`isTTYStdin`)
**Justification:** `runAdopt` inlines `term.IsTerminal(os.Stdin.Fd())` instead of the package's named `isTTYStdin()` owner, duplicating the TTY-detection knowledge. (**Note:** same single line as C-003 viewed from the ownership angle rather than the duplication angle.)

---

## Review: Dimensions E & F

Phase-4 quality + traceability review. Each finding re-verified against source at the cited `file:line`; UX claims settled by building/running `/tmp/dotd-rev`.

---

### E-001 — CONFIRMED
**Re-verified at:** `internal/predicate/eval.go:89-94` (`NewEvaluator` → `NewFuncRegistry(Strict)`, empty `funcs` map at :48), `eval.go:76` (Strict → `unknown function` error), `internal/packages/packages.go:1-2` (doc claims `installed()`/`installable()`); `grep '\.Register(' cmd internal --include='*.go' | grep -v _test.go` → **zero** production registrations.
**Justification:** Only `exists()` is wired inline (`eval.go:161`); the Strict registry is instantiated empty, so any documented `installed()`/`installable()` predicate is a guaranteed hard error — false doc/API contract.

### E-002 — CONFIRMED
**Re-verified at:** `grep -rln 'internal/setup' --include='*.go' | grep -v _test.go` → only `cmd/dotd/teardown_cmd.go`; that file uses only `DetectShellConfig`/`HasSourceLine`/`RemoveSourceLine` (:75,76,131); `grep 'setup\.Scaffold|setup\.Options|setup\.Result|setup\.Action'` non-test → empty; `grep 'setup\.' setup_cmd.go` → empty.
**Justification:** The `Scaffold`/`Options`/`Result`/`Action` headline API has zero production callers; it is a fully-built, unit-tested but unreachable subsystem — confirmed dead exported API.

### E-003 — CONFIRMED
**Re-verified at:** `grep 'AppendSourceLine|setup\.SourceLine|\.SourceLine' --include='*.go' | grep -v _test.go` → only the declaration/comment at `internal/setup/shell.go:81,84,99`; no production caller.
**Justification:** `AppendSourceLine`/`SourceLine` are exported and tested but never invoked in the live path, while `RemoveSourceLine` (teardown) is wired — confirmed asymmetric dead code.

### E-004 — CONFIRMED
**Re-verified at:** `DetectInstalled` → only own decl (`catalog.go:106-107`), no non-test caller; `predicate.And` → only own decl (`ast.go:76`); `.Check` → written once at `packages.go:98`, never read (`grep '\.Check' internal/packages cmd/dotd | grep -v _test.go` shows only the assignment).
**Justification:** All three are inert exported items; the documented `check:` YAML field is parsed then silently ignored (`Installed` only does lookPath), a real no-op config knob. (**Note:** severity Low but `.Check` is a user-visible silent no-op worth surfacing.)

### E-005 — CONFIRMED
**Re-verified at:** `grep ValidateNodes cmd internal | grep -v _test.go` → sole caller `cmd/dotd/main.go:296` (inside `runPipeline`); read commands call `pipeline.Walk`+`pipeline.Order` directly with no `ValidateNodes`: `dag_cmd.go:28,36`, `list_cmd.go:52,62`, `bundle.go:49,59`, `compose_cmd.go:37,66,74`, `package.go:35,77,106`.
**Justification:** Action-sequencing validation runs only on the apply/check driver, so `list`/`dag`/`bundle`/`compose` can present node sets that `apply` would reject — inconsistent validity contract confirmed.

### E-006 — CONFIRMED
**Re-verified at:** `internal/pipeline/walk.go:421-424` doc states `anns must be pre-normalized`; the sole caller normalizes at `walk.go:185` one line-region before `mergeActions(state.actions, anns)` at `walk.go:198`; nothing in the signature/body enforces it.
**Justification:** A correctness-relevant ordering precondition is expressed only in a cross-file doc comment; a second raw caller would silently get wrong action lists — confirmed hidden-coupling smell. (**Note:** contained to one caller today, Low severity.)

### E-007 — CONFIRMED
**Re-verified at:** bare/stage-prefixed errors `walk %s: %w`/`order: %w`/`act: %w` at `main.go:290,305,316` and `walk:`/`order:` at `dag_cmd.go:30,38`; command-prefixed `bundle: ...` at `bundle.go:79,103`; correct `env edit` text vs others.
**Justification:** Same stage prefixes appear in both `main.go` and `dag_cmd.go` with no command qualifier, mixed with command-prefixed errors elsewhere — confirmed prefix-convention inconsistency. (**Note:** Low; no correctness impact. Some config_cmd line numbers in the writeup are slightly off, but the cited messages exist nearby and the claim holds.)

---

### F-001 — CONFIRMED
**Re-verified at:** `cmd/dotd/filter_prompt.go:37-39` — `if errors.Is(err, huh.ErrUserAborted) { os.Exit(1) }` inside `filterWithPrompt`; `grep filterWithPrompt | grep -v _test.go` → 9 command callsites (`main.go:300` apply+check, `compose_cmd.go:41,70`, `dag_cmd.go:32`, `package.go:39,81,110`, `bundle.go:54`, `list_cmd.go:57`).
**Justification:** `os.Exit(1)` from inside a cobra `RunE` chain bypasses the error pipeline (no message, no deferred cleanup), making a user abort indistinguishable from a real exit-1 failure — confirmed.

### F-002 — CONFIRMED
**Re-verified at:** `cmd/dotd/unapply_cmd.go:46` (`out := cmd.OutOrStdout()`), :137-139 (`os.Remove` fail → `ui.Errf(out,...)` then `continue`), :151 (`return nil`).
**Justification:** Remove failures are reported to stdout and the command returns nil regardless, so partial failure exits 0 with the error in captured stdout — confirmed silent-failure / wrong-stream defect.

### F-003 — CONFIRMED
**Re-verified at:** `internal/log/log.go:2-3` contract; `teardown_cmd.go:48,54` `ui.Warnf(out,...)` to stdout; `compose_cmd.go:94,102` `ui.Missingf`/`ui.Wrongf` to `cmd.OutOrStdout()` while summary uses `cfg.log` (stderr) at :108,110.
**Justification:** Diagnostic-class output routes to stdout and compose check splits status across both streams, contradicting the documented contract — confirmed.

### F-004 — CONFIRMED
**Re-verified at:** `prompts.go:16` `promptConfirm` `[y/N]` default-No; `init_cmd.go:165-168` `promptYN` `[Y/n]` default-Yes with `EOF → default yes`; huh forms at `adopt.go` / `filter_prompt.go`.
**Justification:** Three unrelated prompt mechanisms with opposite default capitalization and `init`'s EOF→yes auto-accept on closed stdin — confirmed UX inconsistency.

### F-005 — CONFIRMED
**Re-verified at:** `adopt.go:92` `!term.IsTerminal(os.Stdin.Fd())` inline vs `filter_prompt.go:95-97` `isTTYStdin()` (identical body).
**Justification:** TTY check duplicated inline rather than using the shared helper (and init/setup use EOF fallbacks — a third approach) — confirmed maintenance hazard, no user-visible bug. (**Note:** Low severity.)

### F-006 — CONFIRMED
**Re-verified at:** `config_cmd.go:90` `Short: "Open dotcfg.yaml in $EDITOR"`; :8 `dotcfg` is the import alias for the config package; :96 opens `cfg.configPath` (config.yaml); `env.go:94` correctly says "Open env.yaml".
**Justification:** Help text names a nonexistent file `dotcfg.yaml` (the Go import alias, not a real path) — confirmed minor doc/help defect.

### F-007 — DISCARDED (informational, no defect)
**Re-verified at:** skip lines `integration_test.go:401,420,452,472,495,509,517` carry stale "not yet migrated/implemented to v2" messages; ran `/tmp/dotd-rev compose` → prints usage to stderr, exit 0; compose/package commands run real pipeline logic (`compose_cmd.go:28-115`, `package.go:26-131`, `internal/pipeline/act.go`).
**Justification:** The Phase-1 "stub" lead is refuted — commands are fully wired; only the integration tests are skipped, so this is a coverage gap, not a UX defect. (**Note:** the skipped paths are exactly where F-002/F-003 bugs live, so the gap raises regression risk.)

---

## Review: Dimension T (test quality)

Total `go test ./... -cover` (profile-based): **61.5%**. All per-function coverage figures below re-measured from `/tmp/cov.out`. Skip directives, dead-code, and test-content claims verified against source.

### T-001 — CONFIRMED
**Re-verified:** `Act 42.9%`, `ComposeFileName 0.0%`, `deriveLinkDest 0.0%` — exactly as cited.
**Justification:** The compose branch of `Act` and its helpers are entirely unexercised; correctly cited.

### T-002 — CONFIRMED
**Re-verified:** `deriveLinkDest 0.0%`; `act_test.go` always passes explicit `Dest`.
**Justification:** Empty-Dest link derivation (the `dot-`→`.` / `nosync-` rewrite) has zero coverage.

### T-003 — CONFIRMED
**Re-verified:** `expandDest 50.0%`; only `~/` covered, `~bin` branches not.
**Justification:** Coverage % matches and the `~bin`/`~bin/` expansion path is genuinely untested.

### T-004 — CONFIRMED
**Re-verified:** `createSymlink 33.3%`.
**Justification:** Only the no-pre-existing-file happy path is covered; force/conflict/re-link branches are not.

### T-005 — CONFIRMED
**Re-verified:** `parseDaggerActions 0.0%`, `Walk 46.0%`.
**Justification:** No composition-enabled dir fixture exists; the dir-level compose-target branch is unexercised. (**Note:** overlaps T-001/T-053 as part of the compose-feature gap.)

### T-006 — CONFIRMED
**Re-verified:** `Walk 46.0%`; no `walk_test.go` assertion sources a node from a `files:` dict.
**Justification:** The explicit `files:` dict-entry pass is untested.

### T-007 — CONFIRMED
**Re-verified:** `mergeSortedByName 54.5%`; `order_test.go` has only `AlphaNoAfter`/`AfterConstraint`/`PrefixAfter`/cycle/dup tests.
**Justification:** The simultaneous-multi-ready tie-break interleave the function exists for is never driven.

### T-008 — CONFIRMED
**Re-verified:** `ResolveAfterRef 100.0%` but no test in `order_test.go` constructs a self-matching prefix or unknown ref.
**Justification:** Self-edge guard and unknown-ref skip are reachable but unasserted; borderline-Medium but real. (**Note:** `ResolveAfterRef` itself is fully covered via other callers — the gap is the unknown/self-ref *branches* in Order.)

### T-009 — CONFIRMED
**Re-verified:** all five `Keys` methods, `And`, `collectKeys` at `0.0%` in the predicate package; no `.Keys()`/`And(` in predicate tests.
**Justification:** Key-extraction and the `And()` constructor are untested within the package.

### T-010 — CONFIRMED
**Re-verified:** `filter_test.go` has `AndBothMissing` but no OR-branch test; `OrExpr.Keys 0.0%`.
**Justification:** The documented "OR collects all branches" caveat is unverified. (**Note:** related to T-009.)

### T-011 — CONFIRMED
**Re-verified:** `NewEvaluator 0.0%`, `evalCall 77.8%`.
**Justification:** Canonical constructor bypassed by all tests (struct literals used); nil-registry path untested.

### T-012 — CONFIRMED
**Re-verified:** `Register 66.7%`, `MissingKeyError.Error 0.0%`.
**Justification:** Duplicate-panic branch and the typed error's message are unasserted; Low but real.

### T-013 — CONFIRMED
**Re-verified:** `DefaultBinDir 0.0%`, `DefaultGeneratedDir 0.0%`, `DefaultConfigFile 0.0%`, `XdgConfigHome 0.0%`, `xdgConfigHome 33.3%`; ecosystem pkg 47.7% (lowest).
**Justification:** Canonical default-path endpoints are untested; matches cited coverage exactly.

### T-014 — CONFIRMED
**Re-verified:** `ResolvePath 100.0%` line coverage, but four tests are structurally near-identical (all set `DOTD_TEST_VAR`).
**Justification:** Borderline-Low — line coverage is full; the envVar-name and tilde-non-expansion contracts have no dedicated guard. (**Note:** documentation-as-test gap, prefer surfacing.)

### T-015 — CONFIRMED
**Re-verified:** `Resolve 100.0%` line coverage; no single all-three-layers-on-one-key test, no input-mutation test.
**Justification:** Borderline-Low — thoroughness gap, not a line gap; surfaced per borderline rule.

### T-016 — CONFIRMED
**Re-verified:** `loadFrom 83.3%`; config/env `Save` happy-path only; no unknown-field decode-error test.
**Justification:** The `KnownFields(true)` strictness guardrail and Save error wraps are untested; Low but real.

### T-017 — CONFIRMED
**Re-verified:** all `walk_test.go` calls use `nodes, _, err`; `cascadeState 76.2%`, `combineWhen 85.7%`.
**Justification:** The `disabled` return slice is never inspected; `@disable` exclusion is unverified.

### T-018 — CONFIRMED
**Re-verified:** `mergeActions 76.1%`; no two-explicit-links fixture.
**Justification:** The keep-both-for-validateNode branch is unexercised; Medium and real.

### T-050 — CONFIRMED
**Re-verified:** `SaveYAML 0.0%`; no `_test.go` in `internal/fileutil`.
**Justification:** The atomic-write helper has no direct test; cleanup/rename branches unverified.

### T-051 — CONFIRMED
**Re-verified:** `DetectShellConfig/SourceLine/HasSourceLine/AppendSourceLine` all `0.0%`; `RemoveSourceLine 94.7%`; `AppendSourceLine` has NO non-test prod caller (grep confirms only its own def + a doc-comment mention).
**Justification:** RC-wiring writers are untested and `AppendSourceLine` is effectively dead in prod; correctly cited.

### T-052 — CONFIRMED
**Re-verified:** no `_test.go` in `internal/ui` or `internal/log` (both 0.0%).
**Justification:** Low-severity but accurate; `--log-level` validation logic is genuinely untested in isolation.

### T-053 — CONFIRMED
**Re-verified:** five `t.Skip` directives at integration_test.go:452,472,495,509,517 with stale "not yet implemented" reasons; `hasComposeAction 0.0%`, `composeGenName 0.0%`; no compose e2e script. Compose IS implemented (act.go ActionCompose).
**Justification:** The skip reasons are false and the entire compose feature ships with zero automated coverage; Critical, correctly cited. (**Note:** the single largest gap; overlaps T-001/T-005 from T1.)

### T-054 — CONFIRMED
**Re-verified:** `t.Skip` at integration_test.go:401,420; `loadRegistry 0.0%`; `collectPackageRequests 88.9%`.
**Justification:** `package check`/`generate` have no command-level test; `loadRegistry` glue is untested. (**Note:** stale-skip pattern shared with T-053.)

### T-055 — CONFIRMED
**Re-verified:** `runAdopt 0.0%`, `conventionsFrom 0.0%`, `promptAdoptConfirm 0.0%`; `resolveToFlag 100.0%`.
**Justification:** The `dotd adopt` command path is entirely uncovered; only the pure helper is tested.

### T-056 — CONFIRMED
**Re-verified:** `collectDeps 8.7%`, `shellQuote 0.0%`, `runBundle 65.3%`.
**Justification:** `TestBundleSimple` has no `@after` deps, so bundle's transitive-dep core contract is unexercised; coverage matches.

### T-057 — CONFIRMED
**Re-verified:** `scaffoldDagger 0.0%`, `scaffoldDaggerInteractive 0.0%`, `promptYN 0.0%`.
**Justification:** Only the precondition-guard test exists; the scaffolding/idempotency path is untested; Medium and real.

### T-058 — CONFIRMED
**Re-verified:** `filterWithPrompt 40.0%`, `promptMissingKeys 0.0%`, `printPersistHint 0.0%`.
**Justification:** The headline TTY missing-key prompt path (the reason the feature was written) is entirely untested; coverage matches; High and well-cited.

### T-059 — CONFIRMED
**Re-verified:** `loadConfig 0.0%`, `newConfigEditCmd 11.1%`, `newEnvEditCmd 10.0%`; the `"config"` matches in main_test.go are dotfiles subdir paths (confDir), not commands.
**Justification:** No `config` command test exists and `$EDITOR` edit subcommands are near-zero; correctly cited.

### T-060 — CONFIRMED
**Re-verified:** no `get-os`/`get-hostname` reference in any `cmd/dotd/*_test.go`.
**Justification:** The canonical OS/host detectors (incl. darwin→macos normalization) have no test; Medium and real. (**Note:** compounds with T-065.)

### T-061 — CONFIRMED
**Re-verified:** no `--all` literal in any `cmd/dotd/*_test.go`; all four unapply tests run the default filtered path.
**Justification:** The `unapply --all` branch and the flag-shadow wiring are untested; Medium and real.

### T-062 — CONFIRMED
**Re-verified:** integration_test.go:346-347 contains the `t.Log("...unexpectedly succeeded; skipping...")`/`return` escape hatch exactly as quoted.
**Justification:** The conflict test conditionally abandons its most important (no-force protection) assertion; a real self-skipping-assertion weakness.

### T-063 — CONFIRMED
**Re-verified:** `main_test.go:196` asserts `strings.Contains(out, "symlinks:")` only; no broken-state/negative check test exists.
**Justification:** `check` tests assert summary presence, not drift/conflict detection — the command's actual purpose; Medium and real.

### T-064 — CONFIRMED
**Re-verified:** `run-e2e.sh` resolves `DOTD_VERSION` from latest GitHub release; `procure/release.sh` installs the published binary via `install.sh --version`.
**Justification:** The canonical e2e suite tests a shipped artifact, not HEAD; accurate process-gap finding. (**Note:** compounds T-053/T-062 — only a stale release is hard-tested for conflicts.)

### T-065 — CONFIRMED
**Re-verified:** tests inject `--env os=linux`; env.yaml fixtures are empty.
**Justification:** Borderline-Low — the override path is legitimate, but the real shell-expand → `get-os` resolution chain is never traversed; surfaced. (**Note:** compounds T-060.)
