# Remaining Items — pickup context (as of 2026-06-13, main = aef4956)

Local only — do not commit. Companion to `handoff-2026-06-13b.md` and the two
audit findings docs (`audit-2026-06-12-findings.md`, `audit-2026-06-13-findings.md`).

Both audits' actionable findings are shipped (06-12 → PRs #106–#110;
06-13 → #111–#117, incl. the X-queue in #117). What follows is everything
left, with enough context to start cold.

Priority shorthand: items are ordered by how much they're worth doing, not by
audit priority codes. D6 is the only substantive one; the rest are cosmetic
fix-when-touched.

---

## D6 — Consolidate the two prompt stacks (the one real item)

**Status:** investigated, feasible, NOT started. Recommended as a dedicated PR.

### The situation
`cmd/dotd/prompts.go` contains two unrelated prompt implementations:

- **huh stack** (`newHuhForm`, `promptMenu`, `promptSelect`, `promptInput`,
  `promptBool`, `promptInputs`) — built on `charmbracelet/huh`. TTY → rich
  forms; non-TTY → "accessible mode" (numbered menus, line-buffered text).
  Needs the `byteReader` one-byte-at-a-time wrapper so huh's per-field
  scanners don't over-read piped input.
  Callers: `annotate_cmd.go`, `adopt.go`, `filter_prompt.go`.

- **bufio stack** (`promptConfirm`, `promptDefault`, `promptYN`, `promptPath`,
  + `printField`/`fieldPrompt` layout helpers) — reads lines off a
  `bufio.Reader`, prints its own hand-formatted prompts.
  Callers: `setup_cmd.go`, `init_cmd.go`, `teardown_cmd.go`, `unapply_cmd.go`.

The bufio stack is the original; the huh stack arrived with the annotate
wizard. They never got unified — two mental models for the same job.

### What was confirmed this session (the unknown is gone)
A standalone probe of huh v1.0.0 accessible mode showed it replicates the
bufio behavior without loss:
- Confirm respects the initial value as the Enter-default (`[Y/n]`/`[y/N]`).
- Confirm on EOF → `false` (matches "never auto-accept on closed stdin").
- Text input on empty line / EOF → returns the prefill (matches `promptDefault`).

So `promptConfirm`, `promptYN`, `promptDefault` can each become thin huh
wrappers. No technical blocker. (The earlier "brainstorm first" flag was about
this unknown — now resolved.)

### Why it's still a real PR, not a quick win
1. **Output format changes.** The bufio path renders a bespoke layout (bold
   label, faint description, `›` prompt via `printField`/`fieldPrompt`). huh
   draws its own `Title`/`Description`. setup/init would *look* different — a
   UX decision, not just a mechanical swap. Decide the target look first.
2. **Test driving changes.** setup/init/teardown/unapply interactive tests
   pipe `\n`-delimited lines via `strings.NewReader`/`runWithStdin`. huh
   accessible mode consumes input via `byteReader` + per-field scanners.
   Each interactive test for those four commands needs its input re-driven
   and output assertions re-checked.
3. **`promptPath` glue** (setup_cmd.go): wraps `promptDefault` then does
   `ExpandHome`/`filepath.Abs`. Reimplement around the huh input.

### De-risking notes
- The test helper (`runWithStdin`) merges stdout+stderr into one buffer, so
  moving a prompt's output channel (huh writes to stderr; `promptConfirm`
  currently writes to stdout) won't silently break `Contains()` assertions.
  Worth aligning to the 06-13 channel policy (prompts/diagnostics → stderr)
  while in there.
- `byteReader` exists precisely so accessible-mode forms can be driven by
  piped input — reuse it; don't reinvent.

### Recommendation
Standardize on huh, delete the bufio helpers (`promptConfirm`, `promptDefault`,
`promptYN`, `promptPath`, `printField`, `fieldPrompt`). One input model, one
look, and the `byteReader` caveat stops being a split-brain footgun. Scope as
its own branch. Suggested order: pick the target UX for setup/init's multi-field
flow → port one command (teardown, simplest: a single confirm) end-to-end incl.
its tests → then setup/init/unapply.

---

## P4 leftovers — fix-when-touched (cosmetic, none blocking)

Per the audit these are explicitly "fix only when already editing that file."
Listed with location + the actual fix so they can be knocked out opportunistically.

- **O5 — skip/exists vocab + indent drift.** teardown/init/setup mix
  "skip"/"exists" wording and 2-space-indented status lines inconsistently.
  Normalize the vocabulary and indentation when next touching those files.
  Evidence: `audit-2026-06-13-findings.md` around line 234.

- **C2 — `unapply` is the only aliased command** (`remove`). Either aliases
  are a feature (add obvious ones elsewhere) or noise (drop this one). Tiny;
  decide when touching unapply. (Note: `dag order` now also has a `check`
  alias as of #117, but that's a back-compat alias, a different rationale.)

- **C3 — error-prefix stragglers.** The "who am I at this layer" wrap-prefix
  rule (command-name prefix vs package prefix vs stage-name) mostly holds; a
  few wraps don't follow it. The rule is documented in `audit-guide.md`. Fix
  stragglers only when touched — no sweep.

- **C5 — order.go silently ignores unknown `@after` refs.** `internal/pipeline`
  order step drops references to non-existent nodes without a warning. The fix
  needs a logger plumbed into the pipeline package (it has none today), which
  is why it was deferred — it's a small API change, not a one-liner.

- **D10 — packages manager-loop dedupe.** `internal/packages`
  `Installable`/`resolveInstallCmd` have near-identical manager-iteration
  loops. Extract the shared loop.

- **D11 — files:-dict link-conflict alignment.** Two code paths handle
  conflicting links from a `files:` dict differently: the dict-action path
  silently drops conflicts; the annotation path keeps both so `validateNode`
  can report. Align them (prefer the reporting behavior).

- **gofmt drift.** ~20 untouched files have pre-existing `gofmt` drift; CI lint
  doesn't enforce plain `gofmt`. A one-shot `gofmt -w ./...` PR would clear it
  if wanted — trivial but touches many files, so it's its own thing.

---

## Old TODO.md items — predate the audits

From `.claude/TODO.md`, still open:

- **CONTRIBUTING.md** — deferred until the project has external contributors.
- **Enable GitHub Pages** — *user action*: repo Settings → Pages → Source:
  GitHub Actions. The docs workflow (`.github/workflows/docs.yml`) deploys on
  merge to main once Pages is enabled. Claude can't do this part.
- **`dotd concepts` sub-topic routing** — add `dotd concepts when`,
  `dotd concepts env`, etc. once the flat version is validated with users.
- **Fedora e2e** — Ubuntu e2e exists (PRs #77–78); Fedora was deferred.

Also noted as a loose end (06-12 handoff): `.claude/TODO.md` references
untracked `docs/superpowers/plans/` files — local-only plan docs that aren't
in git, a small consistency risk if TODO.md is ever read fresh.

---

## Pointers
- Findings + per-finding PR map: `audit-2026-06-13-findings.md` (STATUS block).
- Conventions ratified this round (channel policy, ToolD rule, single owners,
  dry-run=skip-writes, wrapped-when invariant): `audit-guide.md`.
- Last two handoffs: `handoff-2026-06-13.md` (the audit), `handoff-2026-06-13b.md`
  (the X-queue close-out).
