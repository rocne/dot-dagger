# Codebase Audit Task

## Mission

Conduct a thorough, structured audit of the `dot-dagger` codebase. Goal: surface every real design flaw, code smell, and ownership violation — with enough traceability that each finding can be independently verified or disputed.

This is not a surface scan. Read every file. Map every relationship. Every claim must be verifiable.

---

## Codebase Overview

`dot-dagger` is a Go CLI tool (`dotd`) for managing dotfile symlinks via a DAG-based pipeline. Key areas:

- **`cmd/dotd/`** — CLI entry points and command implementations (cobra-based)
- **`internal/`** — domain packages:
  - `adopter` — copies/adopts files into managed dotfiles
  - `annotation` — parses `.dagger` annotation files
  - `config` — config file loading and struct
  - `dagger` — `.dagger` file parsing and model
  - `ecosystem` — platform detection and path defaults
  - `env` — env.yaml loading and resolution
  - `fileutil` — filesystem utilities
  - `log` — structured logging
  - `node` — graph node model
  - `packages` — package manager catalog
  - `pipeline` — walk, filter, act, order, initgen (core orchestration)
  - `predicate` — predicate language (lexer, parser, AST, eval)
  - `setup` — shell config detection and rc-file manipulation
  - `ui` — terminal output helpers
- **`test/`** — integration tests

**Critical design invariants (from CLAUDE.md):**
1. All paths resolved once in `resolvePaths()` via `ecosystem.ResolvePath`. Command code reads `cfg.*` only.
2. All env values resolved via `resolveEnv(cfg)`. Command code reads from the returned map only.
3. Violations: calling `Default*()` outside `resolvePaths`, reading `DOTD_*` env vars after resolution, using `runtime.GOOS` or `os.Getenv("SHELL")` outside `cmd/dotd/getters.go`.

---

## Phase 1 — Agent Team: Codebase Mapping

Deploy a mapping agent per top-level module area. Each agent reads all files in its area exhaustively and produces a component map.

**Each map entry includes:**
- File path
- Package and exported symbols
- Responsibility (one sentence)
- Consumed by (callers, with file:line references)
- Consumes (imports/dependencies used, with file:line references)
- Interfaces / contracts it exposes or fulfills

Combine all agent outputs into a single map document: `docs/audit/00-codebase-map.md`.

**Coverage requirement:** Every `.go` file must appear in the map. No gaps.

---

## Phase 2 — Agent Team: Parallel Audit Passes (Code Quality)

Deploy independent agents per audit dimension. Each agent reads the full codebase map (Phase 1 output) and the raw source files before reporting findings.

### Audit Dimensions

#### A. Canonical Resolution Violations
Find every location where a value is obtained through a non-canonical path when a canonical one exists or should exist.

Red flags:
- `runtime.GOOS` or `os.Getenv("SHELL")` called outside `cmd/dotd/getters.go`
- `ecosystem.Default*()` called outside `resolvePaths()`
- `os.Getenv("DOTD_*")` called after resolution has run
- `os.UserHomeDir()` called where `cfg.linkRoot` is already resolved
- Any config or path value derived independently in two places instead of using a shared resolution point

#### B. Magic Values and Constants
Find every hardcoded string, number, or path that:
- Appears in more than one file without a shared constant
- Would be wrong if the value changed but only one callsite was updated
- Is not obviously a universal convention (e.g. `"/"` path separator)

For each: note all locations where the value appears.

#### C. Duplicated Code and Logic
Find every instance of:
- Identical or near-identical blocks across files
- The same decision made independently in two places (e.g., "is this file managed?" checked with different logic in two modules)
- Functions that are wrappers so thin they add no value

#### D. Ownership Violations
Find every location where a module reaches into another module's domain for data or behavior it shouldn't own.

Red flags:
- A formatting/UI module determining a config path
- A pipeline stage directly reading env vars instead of accepting resolved values
- A command directly constructing a type that should be built by a factory
- A module that "knows too much" about the internals of a sibling module

Principle: each piece of information or behavior has exactly one logical home. If you can describe two plausible owners, that is a smell worth examining.

#### E. Design and API Quality
Evaluate:
- Are exported function signatures stable and minimal, or do they expose internals?
- Are error messages user-facing or internal? Are they consistent?
- Is the pipeline composable, or do stages have hidden coupling?
- Do interfaces exist where they should (testability, multiple implementations)? Do they exist where they shouldn't (premature abstraction)?
- Is the DAG ordering logic clearly separated from the execution logic?

#### F. UX and CLI Behavior
Evaluate:
- Are command names, flags, and help text consistent?
- Are error messages actionable?
- Are there user-hostile behaviors (silent failures, missing hints, bad exit codes)?
- Is the output (stdout vs stderr) correct for each command?

---

## Phase 3 — Agent Team: Test Quality Audit

**Runs after Phase 1 completes, in parallel with Phase 2.**

Deploy a dedicated agent team to evaluate test coverage and quality. This is a separate phase because it requires comparing the codebase map (Phase 1) against the test files — a different read pattern than Phase 2. Both Phase 2 and Phase 3 can run simultaneously once the Phase 1 map is available.

Evaluate:
- What behaviors are untested? Cross-reference Phase 1 map against test files.
- Are tests testing behavior or implementation details?
- Are there tests that would pass even if the feature is broken (testing mocks rather than real behavior)?
- Are integration tests representative of real usage?
- Are there test gaps in high-risk areas (e.g. canonical resolution, DAG ordering, predicate eval)?

Output: raw findings (same format as Phase 2 agents) — not yet finalized.

---

## Phase 4 — Review Agent: Quality and Traceability Check

A dedicated review agent reads all raw findings from Phases 2 and 3 and evaluates each one for legitimacy and traceability.

**This phase is about quality, not adversarial filtering.** The goal is to ensure every finding that enters the final report is:
- Grounded in actual code (not a misread or speculation)
- A real problem, not a known legitimate exception or intentional design decision
- Traceable — someone reading the final report can independently verify it

**For each finding, the review agent must:**
1. Re-read the cited source locations independently.
2. Assess whether the finding is:
   - **CONFIRMED** — real problem, correctly cited, no obvious legitimate justification
   - **DISCARDED** — false positive, known exception, misread call graph, or style preference with no correctness impact
3. Record a one-sentence justification for the verdict.

**Speculation rule (applies to Phase 2/3 agents):** A finding must be grounded in actual code — specific file and line. "Probably" and "likely" are not findings and should not have been raised. If a finding from an earlier agent contains no concrete citation, the review agent should DISCARD it as unverifiable.

**Borderline rule (applies to Phase 4 review agent verdicts):** When evidence is real but severity or ownership is genuinely debatable, prefer CONFIRMED with a note flagging the uncertainty. It is better to surface something questionable than to silently drop it. The note must say what is uncertain.

The review agent produces: `docs/audit/review-log.md` — every finding listed with its verdict and justification. Both CONFIRMED and DISCARDED are clearly marked.

---

## Phase 5 — Synthesis Agent: Final Report

A synthesis agent reads `docs/audit/review-log.md` and produces the final audit output.

### Output Files

```
docs/audit/
  00-codebase-map.md         # Phase 1: full component map
  01-canonical-violations.md # Confirmed: resolution path violations
  02-magic-values.md         # Confirmed: hardcoded values
  03-duplication.md          # Confirmed: duplicated code/logic
  04-ownership.md            # Confirmed: ownership violations
  05-design-quality.md       # Confirmed: API and design issues
  06-ux-cli.md               # Confirmed: UX and CLI behavior
  07-test-quality.md         # Confirmed: test coverage and quality
  review-log.md              # All findings: confirmed + discarded with verdicts
  summary.md                 # Executive summary: counts per category, top priority issues
```

### Finding IDs

Each Phase 2 and Phase 3 agent assigns IDs prefixed by its dimension letter and a zero-padded sequence: `A-001`, `A-002`, `B-001`, `B-002`, etc. Test quality agents use prefix `T`. These IDs are stable — the review log and final report reference them by ID. The synthesis agent in Phase 5 assigns a canonical cross-audit ID (`AUDIT-001`, `AUDIT-002`, …) to each CONFIRMED finding in the final category files, and cross-references the original agent ID.

### Finding Format

Each finding in the category files:

```
### [AUDIT-NNN] Short title

**Original ID:** A-001 (or whichever agent raised it)
**Location:** `path/to/file.go:line`
**Severity:** Critical | High | Medium | Low
**Description:** What the problem is.
**Justification:** Why this is a real problem, not a style preference.
**Impact:** What breaks or becomes inconsistent if this is left as-is.
```

**Severity definitions:**
- **Critical** — active correctness hazard; modifying the related value in one place will silently break behavior elsewhere; fix immediately
- **High** — not actively broken but structurally unsound; high risk of future breakage
- **Medium** — violates a design principle but impact is contained
- **Low** — minor inconsistency or style issue; worth noting, low urgency

Discarded items appear only in `review-log.md`, not in category files.

---

## Workflow Requirements

- **Execution order:** Phase 1 first. Phases 2 and 3 run in parallel after Phase 1 completes. Phase 4 after both 2 and 3 complete. Phase 5 last.
- **Use agent teams within phases.** Phase 1 deploys one mapping agent per module area. Phase 2 deploys one agent per audit dimension (A–F). Phase 3 deploys a dedicated test quality team. All agents within a phase run in parallel.
- **Dynamic workflows.** If a mapping agent discovers an area significantly larger than expected, spawn sub-agents rather than truncate output. Completeness is required.
- **No speculation (Phase 2/3 agents).** Every claim must cite a file path and line number. "Probably" and "likely" are not findings.
- **Deduplication (Phase 5 synthesis).** The synthesis agent is responsible for identifying findings that appear in multiple category buckets. Place the finding in the most precise category and cross-reference from any other that would have claimed it. Assign a single canonical AUDIT-NNN ID.
- **Self-contained output.** The `docs/audit/` directory must be complete enough that a reader with no prior context can understand and independently verify every finding.

---

## Output Location

Write all output to: `docs/audit/` (relative to repo root).
