# Session Handoff — 2026-05-18

## What just merged

**PR #63 — `dotd adopt` implementation** (`feature/claude-adopt` → `main`, tag `v0.2.22`)

### Changes

- **`internal/adopter` package** (`internal/adopter/adopter.go`, `adopter_test.go`) — new package owning all adoption logic:
  - `ConventionNames`, `Inference`, `AdoptOptions` types; `DefaultConventions()` helper
  - `Infer(src, info, conv)` — 5-rule inference: executable → `bin/`, shell ext → `shellrc/`, hidden file → `conf/dot-<name>`, config ext → `conf/`, unknown → error hint
  - `Adopt(src, destRel, opts)` — copy, remove original, symlink via `pipeline.Act` with synthetic `RawNode`. Recovery: `os.Rename(destAbs, src)` if Act fails after Remove.
  - `actionsFor` — shellrc→source, bin→link(binDir/name), conf/other→link(original src path)
  - `copyFile` — mkdir-p, O_EXCL create, io.Copy, cleanup on failure

- **`internal/dagger/dagger.go`** — added `ConventionConfig{Shellrc, Bin, Conf string}` and `Conventions ConventionConfig` field to `ComposableNode`. Convention names are now configurable via root `.dagger` file; callers apply defaults for empty fields.

- **`cmd/dotd/adopt.go`** (full replacement of stub) — thin CLI layer:
  - `newAdoptCmd`: `--to` (destination override), `--yes/-y` (skip prompt); no `Hidden: true`
  - `runAdopt`: abs path, stat, load `.dagger` conventions, infer/override dest, TTY-aware huh prompt, dry-run plan print, delegate to `adopter.Adopt`
  - `resolveToFlag`: trailing `/` appends filename, otherwise uses value as-is
  - `conventionsFrom`: DefaultConventions + non-empty overrides from dagger config
  - `promptAdoptConfirm`: `huh.NewConfirm` interactive prompt

- **`cmd/dotd/main_test.go`** — added `TestResolveToFlag` (3 cases)

- **`internal/dagger/dagger_test.go`** — added `TestLoad_Conventions`

- **`go.mod` / `go.sum`** — `charmbracelet/huh` and `charmbracelet/x/term` added as direct deps; Go version bumped to 1.26.1

- **`.claude/docs/spec/cli.md`** — removed "_(not yet migrated to v2)_" from adopt entry

- **`docs/superpowers/specs/2026-05-17-adopt-design.md`** — design spec (committed during brainstorm phase)

- **`docs/superpowers/plans/2026-05-17-adopt.md`** — implementation plan (committed before execution)

---

## State of TODO.md

No deferred items were touched. All items remain as-is.

`dotd adopt` was previously listed as a deferred item ("Stubbed, no plan yet") in the prior handoff doc. It is now fully implemented — that note is no longer accurate and can be disregarded.

---

## Remaining CLI gaps

None. All commands are implemented. `dotd adopt` was the last stub.

---

## Branch state

`feature/claude-adopt` was merged and can be deleted locally:

```sh
git branch -d feature/claude-adopt
```

---

## Spec state

`.claude/docs/spec/cli.md` is current and accurate as of this merge. All other spec files unchanged.

---

## Key design decisions (for future reference)

- **Symlink creation reuses `pipeline.Act`** — adopt builds a synthetic `RawNode` rather than calling `os.Symlink` directly. This ensures adopt behaviour stays consistent with apply.
- **`actionsFor` default is `link(src)`** — for conf/ and any `--to` path, the explicit Dest is the original file path. This correctly handles `--to` overrides without needing `deriveLinkDest`.
- **`ConventionConfig` zero-value = use default** — fields are only overridden when non-empty, so users who don't set `conventions:` in `.dagger` get the standard `shellrc/bin/conf` layout automatically.
- **CLI dry-run short-circuits before `adopter.Adopt`** — prints a plan line and returns; `AdoptOptions.DryRun` field exists for library callers but the CLI doesn't pass it through (intentional layering choice).
