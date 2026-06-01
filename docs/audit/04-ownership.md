# Ownership Violations

Locations where a module owns or reaches into data/behavior that belongs to another module, with no clear single home.

### [AUDIT-022] Prompt/UI helper makes the process-exit decision (`os.Exit(1)`)

**Original ID:** D-001 (merged: F-001)
**Location:** `cmd/dotd/filter_prompt.go:37-39`
**Severity:** High
**Description:** `filterWithPrompt`, a filter-stage wrapper that owns TTY-aware prompting, calls `os.Exit(1)` directly on `huh.ErrUserAborted` instead of returning an error. Process exit is the domain of `main()` — the single canonical exit point is `main.go:26-30`, and every command returns an `error` up the cobra stack (`runCheck` even sets `SilenceErrors` and returns rather than exiting). `grep os.Exit` confirms the only two sites are this and `main.go:28`. Reached from 9 command callsites (`main.go:300` apply+check, `compose_cmd.go:41,70`, `dag_cmd.go:32`, `package.go:39,81,110`, `bundle.go:54`, `list_cmd.go:57`).
**Justification:** A library-ish filter helper hard-terminating the process usurps main's exit ownership, bypasses cobra's error pipeline (no `Error:` message printed, deferred cleanup doesn't run), and is untestable through `Execute()`. The other prompt paths (`promptConfirm`, `promptAdoptConfirm`) correctly return. On abort it should return the error (or a sentinel) and let `main` own the exit code. This is both an ownership violation (D-001) and a UX/exit-code defect (F-001): a user abort is indistinguishable on the wire from a real exit-1 failure.
**Impact:** Ctrl-C/Esc out of a key prompt silently terminates with code 1 and no message; callers never see the abort, no cleanup runs, and "user cancelled" cannot be distinguished from "pipeline error." A latent correctness/testability hazard.
**Cross-reference:** AUDIT-018, AUDIT-053.

### [AUDIT-023] Bidirectional prompt-helper coupling between init_cmd.go and setup_cmd.go

**Original ID:** D-002
**Location:** `cmd/dotd/init_cmd.go:142,164,174` ↔ `cmd/dotd/setup_cmd.go:164,170`
**Severity:** Medium
**Description:** The two interactive-wizard files each own text-prompt primitives the other depends on, in both directions. `init_cmd.go` owns `promptDefault` (`:142`), `promptYN` (`:164`), `expandTildeStr` (`:174`), consumed by setup at `setup_cmd.go:69-121`. Conversely `setup_cmd.go` owns `printField` (`:164`) and `fieldPrompt` (`:170`), consumed by init at `init_cmd.go:100,111`. The package already has `prompts.go`, which owns only `promptConfirm` — making the misplacement explicit.
**Justification:** A primitive used by both siblings has no single logical home; each reaches into the other for it. The named home for shared prompts (`prompts.go`) exists but the init/setup primitives live in two command files instead.
**Impact:** Editing either command file risks breaking the other; the dependency direction is non-obvious. Three distinct prompt styles (`promptConfirm`, the init text style, huh-based `promptAdoptConfirm`) coexist with no consolidation point.
**Cross-reference:** AUDIT-035.

### [AUDIT-024] adopter constructs a partial `pipeline.RawNode` by hand

**Original ID:** D-004
**Location:** `internal/adopter/adopter.go:109-113`, `internal/pipeline/walk.go:41-53` (`RawNode`, 11+ fields)
**Severity:** Medium
**Description:** `adopter.Adopt` reaches into pipeline's domain model and hand-builds a `pipeline.RawNode`, populating only 3 fields (`Path`, `LogicalName`, `Actions`) and feeding it to `pipeline.Act`. `RawNode` is documented as "a file discovered during Walk" and is normally produced exclusively by `pipeline.Walk`. No factory exists for "make a RawNode for a single known file," so adopter must encode knowledge of which subset of fields `Act` reads. Compounding it, the node model is split: package `node` exposes only `DeriveName` while the struct lives in `pipeline`, so adopter must import both to assemble one node.
**Justification:** The owner of `RawNode` construction is `pipeline` (via `Walk`); adopter sidesteps it. If `Act` ever depends on another `RawNode` field, this partial construction silently produces wrong behavior with no compile error.
**Impact:** adopter is coupled to pipeline's internal field-read contract; a pipeline-internal change can break adoption without a type error. The node/pipeline split forces two imports to build one logical entity.

### [AUDIT-025] Generic `fileExists` owned by teardown_cmd.go, reached for by unapply_cmd.go

**Original ID:** D-003
**Location:** `cmd/dotd/teardown_cmd.go:148`, consumed at `cmd/dotd/unapply_cmd.go:105`
**Severity:** Low
**Description:** `fileExists(path string) bool`, a fully generic stat helper, is defined in `teardown_cmd.go` but used by `unapply_cmd.go:105` (`initShExists := fileExists(cfg.initFile)`) as well as teardown itself. A command-agnostic filesystem helper has no business in a specific command file (the package has `internal/fileutil`, currently `SaveYAML`-only).
**Justification:** teardown does not own file-existence semantics — unapply has equal claim, which is the tell that the owner is wrong. (Note: Low — borderline benign same-package sharing, surfaced because the owner file is arbitrary.)
**Impact:** unapply silently depends on teardown's file being present; refactoring teardown breaks unapply. Minor because the helper is trivial.
