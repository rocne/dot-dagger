# Mutation testing (Track E) — deferred to a high-memory host

**Decision (2026-06-15): defer mutation testing off the remote dev box; run it on
the maintainer's Mac.** This doc + `docs/superpowers/scripts/run-mutation-testing.sh`
are committed (not embedded in the binary) specifically so the plan and tooling
travel with the repo to a different machine. Self-contained on purpose.

## Why deferred (do not retry on the remote box)
The remote dev box is **1.6 GB RAM with 0 swap** — intentionally a low-memory dev
machine. `gremlins` defaults `--workers` to `GOMAXPROCS` (one worker per core), and
each worker forks a `go test` compile (~200–400 MB resident). Enough parallel
compiles overrun 1.6 GB; with no swap the kernel OOM-killer fires immediately and
reaps the largest processes — which were the Claude Code harness and the tmux
server. That killed the working session repeatedly before diagnosis. Tuning
`--workers 1` still gambles the live session on a 0-swap box, so mutation testing is
simply the wrong workload for that host.

Secondary issue observed in the one run that completed (`internal/pipeline`):
122 of 125 mutants reported **TIMED OUT** (the "100% efficacy" was meaningless —
only 3 mutants actually ran). The derived per-mutant timeout was too tight for the
fast suite. The runner script sets `--timeout-coefficient 10` to fix this.

## How to run it on the Mac
1. Get the repo on the Mac (clone, or rsync the working tree).
2. Install Go if needed: `brew install go`.
3. From the repo root:
   ```sh
   bash docs/superpowers/scripts/run-mutation-testing.sh
   ```
   The script: installs `gremlins` if missing, asserts a GREEN `go test ./...`
   baseline, then runs mutation testing per priority package with safe settings
   (`--workers 4 --timeout-coefficient 10`, both overridable via env vars), and
   writes `./phaseE-mutation-results-<date>.md` (a local artifact — do not commit).
4. Tune `WORKERS` to the Mac's free RAM: roughly `WORKERS <= floor(free_GB / 0.5)`.
   16 GB → 4–8 is comfortable.

## Goal & what to do with the results
The repo has ~10.5k LOC of tests; coverage ≠ effectiveness. Mutation testing answers
"do the tests actually KILL mutants, or just execute lines?"
- **LIVED** mutant = a mutation the suite failed to catch = an untested behavior.
- **NOT COVERED** = code no test exercises at all.

Triage each real gap by stakes (data-loss / half-applied-state > correctness >
cosmetic) and write the test that would kill the mutant.

## Cross-reference: Track B correctness findings (corroboration)
A read-only correctness audit of the pipeline/predicate engine (Track B, done on the
remote box 2026-06-15) found two bugs by inspection. If mutation testing shows
**surviving mutants at these exact spots, that corroborates the gap** — and the fix
should add a test that both kills the mutant and locks in the corrected behavior:

1. **`internal/predicate/eval.go` (AndExpr / OrExpr).** A `ConditionExpr` on an
   unset env key returns a `MissingKeyError`, and And/Or propagate it eagerly on the
   first operand. So the operators are **not commutative under a missing key**:
   `context=work OR os=linux` errors when `context` is unset even if `os=linux` is
   true, whereas `os=linux OR context=work` succeeds. The interactive (TTY) path
   masks this by pre-prompting for all referenced keys; the non-interactive path
   (CI/scripts) fails hard. Watch for surviving `CONDITIONALS_NEGATION` /
   `INVERT_LOGICAL` mutants here — they'd confirm the missing-key paths are untested.
   Fix needs a semantics decision (defer the error vs treat missing key as false).

2. **`internal/node/node.go` `DeriveName`.** It strips the final component's
   extension via `filepath.Ext`, but for a leading-dot filename `filepath.Ext`
   returns the whole name — `.gitignore` → `""`. Two such files anywhere in a
   dotfiles tree both derive the empty logical name → `Order` aborts the whole
   pipeline with a duplicate-name error. Watch for surviving mutants around the
   `filepath.Ext` / `TrimSuffix` branch.

## Campaign context (where this sits)
Part of a multi-phase code-quality audit campaign. Phase 0 scouts (vuln/dep,
canonical-path) were clean. Track B (pipeline correctness) is complete. Track E
(this) is deferred here. Remaining tracks that suit the low-memory remote box are
read-heavy and compile-free: release-security review, CLI-UX, and applying the B
fixes.
