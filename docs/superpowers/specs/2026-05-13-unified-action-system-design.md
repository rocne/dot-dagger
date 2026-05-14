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

Non-action annotations are not touched. `normalizeActionAnnotations` applies only to the file annotation path (`annotation.Scan` output). Files declared in `.dagger` `files:` dict entries use `parseActionString` directly and are canonical by construction — normalization does not run on them.

Note on `@symlink`: the alias uses parens (`@symlink(dest)`), consistent with all other annotations. Space-separated form (`@symlink dest`) is not supported.

Note on `@action compose` in a file annotation: normalization passes it through unchanged. `ValidateNodes` will reject it as "compose is only valid on directories."

**`ValidateNodes([]RawNode) error`**

Validates every node in the slice. Collects all errors before returning — does not stop at the first. Returns nil if everything is valid.

Error conditions (`<name>` = `n.LogicalName`):

| Condition | Error message |
|---|---|
| `compose` action on a file node | `node <name>: compose is only valid on directories` |
| `link` action with empty dest | `node <name>: link requires a destination` |
| `link` or `source` before `compose` on a dir node | `node <name>: link/source must follow compose` |
| Two `link` actions with different destinations on same node | `node <name>: conflicting link destinations` |

"Before" means lower index in the `Actions` slice. Walk always prepends `compose` at index 0 for directory nodes, so this rule is a future-proofing guard.

Same-dest duplicate `link` actions on the same node are silently deduplicated (consistent with the `seen` map in `mergeActions`).

Directory nodes are identified by `n.ComposeTarget == n.Path`. This works because Walk only emits directory-level nodes for `composition.enabled` dirs, which always set `ComposeTarget = Path`. Empty action lists are valid (no-op nodes pass through silently).

---

### 2. `internal/pipeline/walk.go` (modified)

**`normalizeActionAnnotations` call site:** after `annotation.Scan()`, before `mergeActions`.

**`mergeActions` simplified:** only handles `Key == "action"`. For each such annotation, call `parseActionString(a.Args)` to recover the `Action` struct, then apply the accumulation logic:

- `compose`: add if not already seen
- `link(dest)`: if `link` not seen, append; if already seen, replace dest
- `source`: add if not already seen and `no-source` not already seen
- `no-source`: remove any existing `source` from the accumulated list, then add `no-source`

`no-source` remains in the final `Actions` slice — `act.go` detects it in its first-pass scan to suppress sourcing. `source` and `no-source` never coexist in the final slice: `mergeActions` ensures this invariant.

The individual `case "source"`, `case "link"`, `case "no-source"` branches in the current `mergeActions` switch are removed — all action annotations arrive pre-normalized as `Key == "action"`.

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
- Unit tests for `ValidateNodes`: each of the four error conditions, plus valid sequences (`compose → link`, `compose → source → link`, file with `source` only)
- Existing pipeline integration tests must pass unchanged — aliases still work
