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
- **Surface chosen:** explicit `dotd docs --full` flag (behavioral name, audience-
  neutral — a human wanting the dump uses the same flag). Bare `dotd docs` is
  reserved for the future human view. Help text describes the output as an
  "llms-full-style machine-readable reference."
- **Audience is client-side, binary-only agents.** The target is an agent that has
  only the *installed binary* (brew/curl), no source checkout — which is the entire
  reason to embed docs at all. An agent working inside the repo doesn't need this:
  `docs/` is right there. Therefore the **only** discovery channel is the tool
  itself — `dotd --help`. Repo-level conventions (`AGENTS.md`, `llms.txt`) are
  explicitly out of scope: they're repo files, never shipped in the binary, and
  serve the audience that least needs the feature.
- **Discoverability = help text only** (nothing else travels with the binary):
  root `dotd --help` long text / examples names `dotd docs --full`, and the `docs`
  command's own help labels `--full` as the complete machine-readable reference.
  The binary never writes a file; a caller who wants one redirects
  (`dotd docs --full > x.txt`).

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
`docs --full` is the superset; any dedup between them is a later concern.

## Architecture

Pure `//go:embed`, no codegen or build-time staging.

**The embed-path constraint:** `go:embed` can only reach files at or below the
directive file's own directory. `docs/` lives at module root; `cmd/dotd` is a
subdirectory and cannot embed `../docs`. Resolution: place the embed directive in
a **new root-level package file** (`embed.go`, `package dotdagger`, at the module
root). `docs/` is below the root, so it is reachable.

**This is a deliberate tradeoff, not free.** It introduces a stray Go file at the
module root and a package importable only by the bare module path
(`github.com/rocne/dot-dagger`), which is mildly ugly. The alternatives are worse:
symlinks don't work (`go:embed` refuses to follow them), and a `go:generate`/build
step that stages `docs/` next to `cmd/dotd` reintroduces the exact codegen the
goal forbids. The root-package idiom is the standard Go answer for embedding
repo-root assets; accepted as-is.

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

Two renderers with **separate responsibilities** — not one unit doing both. The
earlier draft folded prose concatenation and live cobra introspection into a
single `RenderAgent(fs, *cobra.Command)`, which coupled `internal/docs` to cobra
and violated single-purpose. Split:

1. **Root package `dotdagger`** (`embed.go`) — owns `DocsFS` only. No logic.
2. **`internal/docs`** — *prose only*, FS-in / string-out, no cobra import:
   `func RenderProse(fs fs.FS) (string, error)`.
   - Walks the embedded FS in **deterministic order**: `index.md` →
     `getting-started/` → `concepts/` → `reference/` (alphabetical within each).
   - Emits a leading **index** (llms.txt-style list of the sections that follow),
     then each file body prefixed with a separator header, e.g.
     `# === docs/concepts/conditions.md ===`.
   - Pure: takes only the FS (no globals, no cobra) → testable in isolation.
3. **CLI-help renderer** (in `cmd/dotd`, e.g. `renderCommandRef(root *cobra.Command)
   string`) — walks the cobra tree rendering each command's help, skipping
   `help`/`completion`/hidden commands so the section stays signal. Lives next to
   cobra where the command tree already does.
4. **`cmd/dotd/docs_cmd.go`** — the cobra `docs` command. On `--full`, composes
   `docs.RenderProse(dotdagger.DocsFS)` + `renderCommandRef(rootCmd)` to stdout.
   Bare `dotd docs` falls through to cobra's own subcommand help (which already
   lists `--full`) — no custom stub.

### Data flow

`docs/*.md` (authored) → `//go:embed` at compile → `dotdagger.DocsFS` →
`docs.RenderProse(DocsFS)` + `renderCommandRef(rootCmd)` composed in `docs_cmd` →
stdout. CI release builds embed a fresh tree; `go install` from source embeds the
current working tree. Size impact: ~72K of markdown + rendered help → negligible.

## Command surface

- `dotd docs --full` → full consolidated blob (raw markdown) to stdout: embedded
  prose + generated CLI reference. Help/long text labels it the llms-full-style
  machine-readable reference and is named in root `dotd --help` examples.
- `dotd docs` (no flag) → cobra's own subcommand help (which lists `--full`). No
  custom stub; the namespace is reserved for the human follow-up
  (`dotd docs <topic>`, `dotd docs --list`) without pre-building it badly.
- Unknown flags / subcommands hard-fail (cobra default).

## Error handling

- Embedding is compile-time: a missing/renamed doc path fails the **build**, not
  runtime — staleness surfaces in CI, not in front of a user/agent.
- `RenderProse` is string assembly over an in-memory FS; the only error path is a
  malformed embedded FS (a build-time impossibility), returned as `error` for
  completeness rather than expected at runtime.

## Testing

- `--full` output contains each included file's **path-header** separator (e.g.
  `# === docs/concepts/conditions.md ===`) — assert on stable paths, not on doc
  content that breaks on every edit.
- `--full` output contains **each** top-level command name (CLI reference walked).
- `--full` output contains **no** `docs/superpowers/` path header (exclusion holds).
- `--full` output emits sections in the **exact intended order**
  (index → getting-started → concepts → reference → CLI reference) — assert the
  specific order, not mere run-to-run stability (embed.FS is always stable, so a
  stability check proves nothing).
- Bare `dotd docs` prints cobra subcommand help (lists `--full`), not the blob.
- `RenderProse` is unit-tested against a small in-memory `fstest.MapFS`, isolated
  from cobra and the real embed.

## Out of scope (follow-ups, noted to keep doors open)

- Human `dotd docs <topic>` rendered/paged view + `dotd docs --list` index.
- Man-page generation via cobra `GenManTree`.
- Emitting an `llms.txt` file artifact (already trivially `dotd docs --full > f`).
- Dedup between `dotd concepts` and the embedded `docs/concepts/`.
- Terminal markdown rendering for the human surface.
