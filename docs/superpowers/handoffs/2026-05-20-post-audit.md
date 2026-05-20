# Session Handoff — 2026-05-20

## What just merged

**PR #64 — spec/code audit** (`feature/claude-audit-fixes` → `main`, tag `v0.2.23`)

Full audit of `.claude/docs/spec/` against `internal/` and `cmd/dotd/`, followed by
corrections in both. Audit report: `docs/superpowers/audits/2026-05-18-spec-code-audit.md`.

---

## Spec fixes

- **dag.md §6**: Complete rewrite — flat `.dagger` schema (no `dotd:`/`link:` nesting),
  map-based `files:` dict, all annotation fields documented (`after`, `require`, `request`,
  `disable`, `actions`, `name`, `when`)
- **Global rename**: `.dotd.yaml` → `.dagger` across all spec files
- **compose.md**: Explicit `actions:` required (no inference from convention dir), compose
  targets can live anywhere, correct generated dir path (`~/.local/share/` not `~/.config/`),
  remove `dotd compose apply` and `@retain-prefix`
- **architecture.md**: Accurate package list matching actual `internal/` structure, correct
  dependency table, fixed design decision rows
- **predicates.md**: Replace false auto-detection claim with `$(dotd get-os)` pattern;
  remove TTY-prompt claim (always halts with hint)
- **cli.md**: Add `dotd bundle`, `--log-level`/`--quiet`, remove `--verbose`
- **env.md**: Document `dotd init` pre-population of `os`/`shell` keys
- **actions.md**: Convention dirs are naming + prepopulated `.dagger` defaults, not implicit magic
- **Deleted**: `package-manifests.md` (§20 dropped — manifest system superseded by BasicNode)

---

## Code fixes

- **H1** — `compose: true` alias + `IsCompose()` in `dagger.go`. Both `compose: true` and
  `composition:\n  enabled: true` now accepted.
- **H4** — `@disable` implemented in `walk.go`. Walk signature changed to
  `([]RawNode, []string, error)` — second return is disabled paths. All 9 callers updated.
  Disabled paths logged at debug. `files:` dict supports `disable: true` too.
- **H5/H6** — `deriveLinkDest` now strips `nosync-` and applies `dot-→.` on every path
  component, not just the first.
- **H7** — `BasicNode` expanded with `After`, `Require`, `Request`, `Disable` fields.
  `files:` dict entries now wire all four. `internal/manifest` deleted (was dead code).
  `collectPackageRequests` reads `n.Require`/`n.Request` from `RawNode` directly — no more
  annotation re-scan.
- **H8** — Compose output name derivation: strips `nosync-` before `dot-` before `.d`.
- **M1** — `cfg.Name` applied to compose-target dir nodes in walk (was only applied to
  `files:` dict entries).
- **M6** — `warnIfNosyncUnignored` added to `main.go`, called from both `runApply` and
  `runCheck`. Uses `git check-ignore --quiet`; silent if not a git repo.
- **M9** — `mergeActions` fixed: annotation overrides inherited default link dest (replace
  in-place), but two explicit annotations with different dests are both kept — `validateNode`
  fires the conflict error. `linkFromDefault` bool tracks the distinction.
- **L6** — Check state `"not-a-symlink"` renamed to `"conflict"`.

---

## Key design decisions (for future reference)

- **Every annotation expressible in `.dagger`** — `@when`, `@after`, `@name`, `@action`,
  `@require`, `@request`, `@disable` all have equivalent fields in `files:` dict. This is a
  core design principle: non-annotatable files (binary, JSON, Lua) get full parity via `.dagger`.
- **Walk returns disabled paths separately** — `([]RawNode, []string, error)` rather than
  filtering inside Walk. Callers that don't care use `_`; `runPipeline` logs them at debug.
- **`mergeActions` link override semantics** — annotation overrides inherited default (tracked
  via `linkFromDefault`); two explicit annotations with different dests are a user error caught
  by `validateNode`. This preserves the useful "file overrides dir default" behaviour while
  making true conflicts visible.
- **nosync warning uses `git check-ignore`** — simplest correct approach; no custom gitignore
  parser. Fails silently if git unavailable or not a git repo.
- **`internal/manifest` dropped** — the manifest system (§20) was dead code; the right solution
  was completing `BasicNode` so non-annotatable files can declare `require`/`request` in `.dagger`.

---

## Deferred items (in TODO.md)

- **M2** — `SkipDir` optimization: directory `when` currently cascades into children rather than
  short-circuiting traversal. Behavioural output is identical; not worth the complexity now.
- **M3** — TTY-aware missing-key prompt: currently always halts with hint message. `annotateKeyError`
  would need TTY detection + interactive `dotd env set` flow.
- **M8** — `dotd init` rc-file check: `internal/setup` has `AppendSourceLine`/`HasSourceLine`
  but `init_cmd.go` never calls them. The feature exists at library level; CLI wiring deferred.

---

## State of main

All spec files accurate as of this merge. All CLI commands implemented. Tests green.
Next logical work: M3 (missing-key UX), M8 (init rc-file), or the Docker integration testing
(blocked on repo going public).
