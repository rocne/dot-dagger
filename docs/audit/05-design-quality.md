# Design and API Quality

Exported-API surface issues, dead/unwired code, hidden coupling, inconsistent contracts, and error-message conventions.

### [AUDIT-026] Custom predicate functions unreachable: registry wired empty, doc claims unregistered functions

**Original ID:** E-001
**Location:** `internal/predicate/eval.go:85-94` (`NewEvaluator` → `NewFuncRegistry(Strict)`, empty `funcs`), `internal/predicate/eval.go:76` (Strict → `unknown function` error), `internal/packages/packages.go:1-2` (doc)
**Severity:** High
**Description:** `NewEvaluator` (the only Evaluator constructor) hard-wires a `Strict` `FuncRegistry` with zero registered functions. The only callable predicate function is the inline built-in `exists()` (eval.go:161). Every other `@when foo(arg)` returns `predicate: unknown function "foo"`. Meanwhile the `packages` package doc claims it "provides the installed() and installable() predicate functions" and `Installed`/`Installable` are implemented — but `grep '\.Register('` across non-test code returns zero registrations, so they are never exposed.
**Justification:** A Strict registry with an empty funcs map means any documented `@when installed(git)` is a guaranteed hard error — a false doc/API contract. The entire `Register`/`Warn`/`SetWarnOutput`/`Mode` machinery has no production wiring.
**Impact:** Documented predicate functions silently fail at runtime; users following the package doc hit unexplained errors. A whole sub-API exists solely to be instantiated empty.
**Cross-reference:** AUDIT-045.

### [AUDIT-027] Entire `internal/setup` scaffold API is dead: `Scaffold`/`Options`/`Result`/`Action` have no production caller

**Original ID:** E-002
**Location:** `internal/setup/setup.go:15-59`, sole consumer `cmd/dotd/teardown_cmd.go`
**Severity:** High
**Description:** `internal/setup` is imported by exactly one production file — `teardown_cmd.go` — which uses only `DetectShellConfig`, `HasSourceLine`, `RemoveSourceLine`. The headline API — `Scaffold(opts Options) (*Result, error)` plus `Options`, `Result`, `Action`, `ActionKind`, the `KindDir`/`KindFile`/`StateCreated`/`StateSkipped` consts — has zero non-test callers. The actual `dotd setup` command reimplements config writing inline and never calls `setup.Scaffold`.
**Justification:** A fully-built, documented, unit-tested subsystem (`setup_test.go` exercises `Scaffold` extensively) that no shipped command path reaches; `dotd setup`/`dotd init` duplicate its responsibility.
**Impact:** Carrying cost — tests run against code no binary uses; future contributors may extend the wrong implementation; two divergent scaffold paths can drift.
**Cross-reference:** AUDIT-028, AUDIT-040.

### [AUDIT-028] `AppendSourceLine` / `SourceLine` exported but unused in production

**Original ID:** E-003
**Location:** `internal/setup/shell.go:46-96` (`SourceLine` `:49`, `AppendSourceLine` `:84`)
**Severity:** Medium
**Description:** `AppendSourceLine` and `SourceLine` are exported and tested but have no production caller; only `HasSourceLine` and `RemoveSourceLine` are wired (teardown). `RemoveSourceLine`'s doc references the header "written by AppendSourceLine", implying the writer side once existed in the live path but was replaced — leaving the writer half orphaned.
**Justification:** The RC source line must be written somewhere for teardown's removal to be meaningful, yet the writer is not `AppendSourceLine`. The exported pair is asymmetric dead code.
**Impact:** API-surface smell; confusion about which code actually installs the RC hook.
**Cross-reference:** AUDIT-027, AUDIT-040.

### [AUDIT-029] `ValidateNodes` runs only on apply/check; dag/list/bundle/compose/package skip validation

**Original ID:** E-005
**Location:** `cmd/dotd/main.go:296` (sole production call site, inside `runPipeline`); read commands skip it: `dag_cmd.go:28,36`, `list_cmd.go:52,62`, `bundle.go:49,59`, `compose_cmd.go:37,66,74`, `package.go:35,77,106`
**Severity:** Medium
**Description:** `pipeline.ValidateNodes` ("checks every node for action sequencing errors") has a single production caller inside `runPipeline`, which backs apply/check/unapply/teardown. The other read commands build the pipeline via `pipeline.Walk` + `pipeline.Order` directly and never invoke `ValidateNodes`.
**Justification:** A dotfile with an action-sequencing conflict errors under `dotd apply` but passes silently under `dotd list`/`dag check`/`bundle`. Validation is an implicit, inconsistent contract bolted onto one driver rather than a property of the pipeline stages.
**Impact:** Different commands report different validity for the same repo state; read commands can present node sets that apply would reject, undermining `list`/`dag`/`compose check` as diagnostic tools.

### [AUDIT-030] `mergeActions` carries an unenforced "must be pre-normalized" precondition

**Original ID:** E-006
**Location:** `internal/pipeline/walk.go:421-424` (consumer doc), `internal/pipeline/walk.go:185` (sole caller's normalize) → `:198` (`mergeActions` call), `internal/pipeline/actions.go:13` (`normalizeActionAnnotations`)
**Severity:** Low
**Description:** `mergeActions` documents "anns must be pre-normalized by normalizeActionAnnotations — only Key==\"action\" entries are processed." The precondition is satisfied only because the sole caller happens to call `normalizeActionAnnotations` one line region before the `mergeActions` call. Nothing in the signature or body enforces it, and the normalizer lives in a different file.
**Justification:** A correctness-relevant ordering precondition is expressed only in a cross-file doc comment; a second raw caller would silently get wrong action lists (legacy keys ignored).
**Impact:** Fragile to refactoring; a second caller is a latent silent bug. Contained to one caller today, hence Low.
**Cross-reference:** AUDIT-016.

### [AUDIT-031] Dead exported helpers: `packages.DetectInstalled`, `predicate.And`, `PackageEntry.Check`

**Original ID:** E-004
**Location:** `internal/packages/catalog.go:106-107` (`DetectInstalled`), `internal/predicate/ast.go:76` (`And`), `internal/packages/packages.go:74-98` (`PackageEntry.Check`)
**Severity:** Low
**Description:** Three exported items have no production consumer: `DetectInstalled` (live `setup.go` uses `packages.Catalog` directly), `predicate.And([]Expr) Expr`, and the `PackageEntry.Check` field (assigned during unmarshal at `packages.go:98` but never read — `Installed` only does `lookPath`).
**Justification:** `PackageEntry.Check` is the most notable: a documented YAML field ("custom shell expression to test for installation") that users can set but that has no effect — a silent no-op config knob. The other two are inert exported helpers. (Note: Low, but `.Check` is a user-visible silent no-op worth surfacing.)
**Impact:** `check:` in packages.yaml is accepted and ignored, misleading users. Dead exports inflate the surface and invite incorrect assumptions.

### [AUDIT-032] Inconsistent error-message prefixing in `cmd/dotd`

**Original ID:** E-007
**Location:** `cmd/dotd/main.go:290,305,316` (`walk %s:`/`order:`/`act:`), `cmd/dotd/dag_cmd.go:30,38` (`walk:`/`order:`), `cmd/dotd/bundle.go:79,103` (`bundle:`), and command-prefixed errors in `adopt.go`/`teardown_cmd.go`/`setup_cmd.go`/`env.go`
**Severity:** Low
**Description:** `internal/*` packages are uniformly prefixed `pkgname:`. `cmd/dotd` is inconsistent: command-prefixed errors (`bundle:`, `adopt:`, `teardown:`) coexist with bare/stage-prefixed ones. The same `walk:`/`order:`/`act:` stage prefixes appear in both `main.go` and `dag_cmd.go` with no command qualifier, so a user cannot tell which command produced an error.
**Justification:** The gap is specifically the prefix/qualifier convention (capitalization is consistently lowercase, `%w` wrapping is widespread). (Note: Low, no correctness impact; some config_cmd line numbers in the source writeup are slightly off but the cited messages exist nearby and the claim holds.)
**Impact:** Minor UX inconsistency; harder to attribute errors to a command. No correctness effect.
