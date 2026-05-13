# Unified Action System — Design

**Date:** 2026-05-13  
**Spec reference:** `.claude/docs/spec/actions.md` (§22)

---

## Goal

Wire the `@action <type>` annotation and make action handling fully canonical throughout the pipeline. All legacy annotation aliases (`@source`, `@no-source`, `@link`, `@symlink`) are normalized to `@action` form at a single conversion point. Sequencing validation is added as an explicit pipeline stage.

---

## Pipeline change

```
Walk → ValidateNodes → Filter → Order → Act
```

`ValidateNodes` is the new stage, inserted between Walk and Filter in both `runApply` and `runCheck`.

---

## Components

### 1. `internal/pipeline/actions.go` (new file)

**`normalizeActionAnnotations([]annotation.Annotation) []annotation.Annotation`**

Converts all action-related annotation keys to canonical `{Key: "action", Args: "..."}` form. Called immediately after `annotation.Scan()` in `walk.go`, before `mergeActions`.

| Input annotation | Output |
|---|---|
| `@action ...` | pass through |
| `@source` | `@action source` |
| `@link(dest)` | `@action link(dest)` |
| `@symlink(dest)` | `@action link(dest)` |
| `@no-source` | `@action no-source` |
| `@when`, `@after`, `@name`, etc. | pass through unchanged |

Non-action annotations are not touched.

**`ValidateNodes([]RawNode) error`**

Validates every node in the slice. Collects all errors before returning — does not stop at the first. Returns nil if everything is valid.

Error conditions:

| Condition | Error message |
|---|---|
| `compose` action on a file node | `node <name>: compose is only valid on directories` |
| `link` action with empty dest | `node <name>: link requires a destination` |
| `link` or `source` before `compose` on a dir node | `node <name>: link/source must follow compose` |
| Two `link` actions with different destinations on same node | `node <name>: conflicting link destinations` |

Directory nodes are identified by `n.ComposeTarget == n.Path` (set by Walk for `composition.enabled` dirs). Empty action lists are valid (no-op nodes pass through silently).

`source`/`no-source` conflict is resolved in `mergeActions`, not validation: when `no-source` is encountered, any existing `source` is removed from the accumulated list. The two never coexist in the final `Actions` slice.

---

### 2. `internal/pipeline/walk.go` (modified)

- After `annotation.Scan()`, call `normalizeActionAnnotations` before passing annotations to `mergeActions`.
- `mergeActions` simplified: only handles `Key == "action"`. Removes the individual `case "source"`, `case "link"`, `case "no-source"` branches — all arrive pre-normalized.

---

### 3. `cmd/dotd/main.go` (modified)

Add `ValidateNodes` call after `Walk` in both `runApply` and `runCheck`:

```go
nodes, err := pipeline.Walk(cfg.files)
if err != nil { return err }

if err := pipeline.ValidateNodes(nodes); err != nil { return err }

active, err := pipeline.Filter(nodes, resolved)
```

---

## Aliases that remain valid

All existing annotation forms continue to work — normalization handles them transparently:

| Write this | Works as |
|---|---|
| `@source` | `@action source` |
| `@no-source` | `@action no-source` |
| `@link(dest)` | `@action link(dest)` |
| `@symlink(dest)` | `@action link(dest)` |
| `@action source` | canonical |
| `@action link(dest)` | canonical |
| `@action no-source` | canonical |
| `@action compose` | canonical |

`@require` and `@request` are not actions — they pass through normalization unchanged.

---

## What is NOT in scope

- Convention dir defaults (`shellrc/`, `conf/`, `bin/`) — P3, separate feature
- `actions:` single-string shorthand in `.dagger` files — already works via `parseDaggerActions`
- `compose: true` → `actions: [compose]` alias in `.dagger` YAML — separate concern, existing `composition.enabled` mechanism unchanged

---

## Testing

- Unit tests for `normalizeActionAnnotations`: each alias maps correctly, non-action annotations pass through
- Unit tests for `ValidateNodes`: each of the five error conditions, plus valid sequences (`compose → link`, `compose → source → link`, file with `source` only)
- Existing pipeline integration tests must pass unchanged — aliases still work
