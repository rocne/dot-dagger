# Embed docs into the binary — agent-facing reference

**Date:** 2026-06-15
**Status:** Design approved, ready for implementation plan
**Topic:** Embed the authored `docs/` set into the `dotd` binary and expose a
machine-readable, agent-oriented reference command.

## Goal

Make the full conceptual + reference documentation available *from inside the
`dotd` binary* — offline, self-contained, no doc-site round-trip — so an agent
(or, later, a human) can read it directly from the tool. Docs are authored once
in `docs/` and reflected into the binary automatically at build time. No
hand-maintained Go string literals, no manual copy step.

## Background / decisions

- **Primary consumer: agents.** Humans are a known follow-up, not this scope.
  The agent surface is one large consolidated blob; the human surface (rendered,
  per-topic, paged) layers on later under the same command namespace.
- **Convention research.** No established standard exists for "a CLI binary
  serving its own docs to agents." Adjacent real conventions: `llms-full.txt`
  (a single consolidated doc blob for LLMs — OpenAI Codex ships one) and
  `AGENTS.md` (repo-level agent *instructions*, a different layer). We adopt the
  *format idea* of `llms-full.txt` (one concatenated blob) but do **not** coin a
  new command name. Agent-CLI best practice (non-TTY → machine output, hard-fail
  unknown commands, examples in help) is style guidance we already follow via
  cobra.
- **Man pages considered and deferred.** Man pages are a *renderer* of the
  command tree (cobra `GenManTree`), not a replacement: they omit hand-written
  conceptual prose, install outside the binary, and target the human shell.
  Embedding is the more general primitive — man pages, a human topic view, and an
  emitted `llms.txt` file all become thin renderers over the same embedded source
  and cobra tree. They join the follow-up list; not in scope here.
- **Surface chosen:** explicit `dotd docs --agent` flag. Bare `dotd docs` is
  reserved for the future human view; the agent must pass `--agent`. Help text
  describes the output as an "llms-full-style machine-readable reference" so an
  agent scanning help recognizes it.

## Scope — what goes in the blob

**Included** ("what the tool is and how to use it"):
- `docs/index.md`, `docs/concepts/`, `docs/getting-started/`, `docs/reference/`
  — the mkdocs prose, *excluding* `docs/superpowers/` (internal specs/plans).
- **Generated** per-command help: a walk of the cobra command tree rendering
  each command's usage/help, appended as a "CLI Reference" section. Generated so
  it never drifts from the actual flags.

**Excluded** (development / duplication):
- `docs/superpowers/**` — internal, never named in the embed patterns.
- `CONTRIBUTING.md` — dev-process, not runtime usage.
- `README.md` — near-duplicate of `docs/index.md`; the "what it is" front door
  already lives in the embedded `docs/index.md`. (Revisit if agents miss install
  prose.)

`dotd concepts` (existing hardcoded single-page quick-ref) is left untouched.
`docs --agent` is the superset; any dedup between them is a later concern.

## Architecture

Pure `//go:embed`, no codegen or build-time staging.

**The embed-path constraint:** `go:embed` can only reach files at or below the
directive file's own directory. `docs/` lives at module root; `cmd/dotd` is a
subdirectory and cannot embed `../docs`. Resolution: place the embed directive in
a **new root-level package file** (`embed.go`, `package dotdagger`, at the module
root). `docs/` is below the root, so it is reachable.

```go
// embed.go (module root, package dotdagger)
package dotdagger

import "embed"

//go:embed docs/index.md docs/concepts docs/getting-started docs/reference
var DocsFS embed.FS
```

Exclusion is by **explicit include patterns** — `docs/superpowers/` is simply
never listed, so it can never be shipped. Self-documenting; no embed-all-then-
filter risk of leaking internal material.

### Components (small, independently testable units)

1. **Root package `dotdagger`** (`embed.go`) — owns `DocsFS` only. No logic.
2. **`internal/docs`** — blob assembly. Signature roughly:
   `func RenderAgent(fs embed.FS, root *cobra.Command) (string, error)`.
   - Walks the embedded FS in **deterministic order**: `index.md` →
     `getting-started/` → `concepts/` → `reference/` (alphabetical within each).
   - Emits a leading **index** (llms.txt-style list of the sections that follow),
     then each file body prefixed with a separator header, e.g.
     `# === docs/concepts/conditions.md ===`.
   - Appends a **CLI Reference** section: walks `root` and renders each command's
     help text (cobra `UsageString()` / equivalent), recursively.
   - Pure assembly; takes the FS and command tree as inputs (no globals →
     testable in isolation).
3. **`cmd/dotd/docs_cmd.go`** — the cobra `docs` command. Wires
   `dotdagger.DocsFS` + the root command into `docs.RenderAgent`. Handles the
   `--agent` flag and the bare-command stub.

### Data flow

`docs/*.md` (authored) → `//go:embed` at compile → `dotdagger.DocsFS` →
`docs.RenderAgent(DocsFS, rootCmd)` concatenates prose + generated CLI help →
stdout. CI release builds embed a fresh tree; `go install` from source embeds the
current working tree. Size impact: ~72K of markdown + rendered help → negligible.

## Command surface

- `dotd docs --agent` → full consolidated blob (raw markdown) to stdout.
  Help/long text labels it the agent / llms-full-style reference.
- `dotd docs` (no flag) → short stub: a one-line topic list plus
  "run `dotd docs --agent` for the full reference." Reserves the namespace for
  the human follow-up (`dotd docs <topic>`, `dotd docs --list`) without shipping a
  half-built human renderer now.
- Unknown flags / subcommands hard-fail (cobra default).

## Error handling

- Embedding is compile-time: a missing/renamed doc path fails the **build**, not
  runtime — staleness surfaces in CI, not in front of a user/agent.
- `RenderAgent` is string assembly over an in-memory FS; the only error path is a
  malformed embedded FS (a build-time impossibility), returned as `error` for
  completeness rather than expected at runtime.

## Testing

- `--agent` output contains a known marker string from **each** included doc file.
- `--agent` output contains **each** top-level command name (CLI Reference walked).
- `--agent` output contains **no** `superpowers/` content (exclusion holds).
- Ordering is deterministic and stable across repeated runs.
- Bare `dotd docs` prints the stub (topic list + `--agent` hint), not the blob.

## Out of scope (follow-ups, noted to keep doors open)

- Human `dotd docs <topic>` rendered/paged view + `dotd docs --list` index.
- Man-page generation via cobra `GenManTree`.
- Emitting an `llms.txt` file artifact (already trivially `dotd docs --agent > f`).
- Dedup between `dotd concepts` and the embedded `docs/concepts/`.
- Terminal markdown rendering for the human surface.
