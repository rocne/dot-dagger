# TTY-Aware Missing Key Prompt (M3)

**Date:** 2026-05-31  
**Status:** Approved

## Problem

When a predicate references an env key not set in `env.yaml` or `--env` overrides, `dotd` halts with a hint. This is correct for CI/non-interactive use. In a terminal session it is unnecessarily disruptive — the user has to re-run the command after manually editing `env.yaml`.

## Goal

When stdin is a TTY and one or more env keys are missing, collect all missing keys upfront, prompt for their values interactively, and proceed. Values are used for the current run only. The persist hint is printed immediately after the prompt, before the command does its work. Non-TTY behavior (CI, pipes) is unchanged.

## Scope

All commands that evaluate predicates: `apply`, `check`, `list`, `dag`, `bundle`, `package`, `compose`. The prompt fires wherever `pipeline.Filter` is called.

## Architecture

Three moving parts:

### 1. `pipeline.CollectMissingKeys`

New function in `internal/pipeline/filter.go`. For each node, parses the predicate expression and calls `.Keys()` on the AST — no evaluation, no short-circuiting. Checks which referenced keys are absent from env. Returns all distinct missing keys in encounter order.

`Filter` is unchanged.

```go
func CollectMissingKeys(nodes []RawNode, env map[string]string) ([]string, error) {
    seen := map[string]bool{}
    var keys []string
    for _, n := range nodes {
        if n.EffectiveWhen == "" {
            continue
        }
        parsed, err := predicate.Parse(n.EffectiveWhen)
        if err != nil {
            return nil, fmt.Errorf("filter: node %q: %w", n.LogicalName, err)
        }
        for _, k := range parsed.Keys() {
            if _, ok := env[k]; !ok && !seen[k] {
                seen[k] = true
                keys = append(keys, k)
            }
        }
    }
    return keys, nil
}
```

Using the AST `.Keys()` method (already implemented on every node type in `ast.go`) guarantees all referenced keys are found regardless of AND/OR structure — no short-circuit risk.

Parse errors are surfaced here and returned immediately, same as `Filter`.

### 2. `filterWithPrompt` in `cmd/dotd`

New file `cmd/dotd/filter_prompt.go`. Accepts `isTTY bool` and `stdin io.Reader` separately so the TTY check is injectable and the function is testable without a real file descriptor.

```go
func filterWithPrompt(
    nodes []pipeline.RawNode,
    resolved map[string]string,
    isTTY bool,
    stdin io.Reader,
) ([]pipeline.RawNode, error) {
    if !isTTY {
        active, err := pipeline.Filter(nodes, resolved)
        return active, annotateKeyError(err)
    }

    missing, err := pipeline.CollectMissingKeys(nodes, resolved)
    if err != nil {
        return nil, err
    }
    if len(missing) == 0 {
        return pipeline.Filter(nodes, resolved)
    }

    filled, err := promptMissingKeys(missing, stdin)
    if err != nil {
        return nil, err // Ctrl+C or form error → abort
    }

    printPersistHint(filled) // printed before command output

    augmented := maps.Clone(resolved)
    for k, v := range filled {
        augmented[k] = v
    }

    active, err := pipeline.Filter(nodes, augmented)
    return active, annotateKeyError(err)
}
```

`promptMissingKeys` builds a `huh.NewForm` with one `huh.NewInput` per key. Each input has a `Validate` that rejects empty strings — user must fill a value or Ctrl+C to abort.

### 3. Callsite updates

Every `pipeline.Filter(nodes, resolved)` + `annotateKeyError(err)` pair is replaced with:

```go
isTTY := term.IsTerminal(int(os.Stdin.Fd()))
active, err := filterWithPrompt(nodes, resolved, isTTY, os.Stdin)
if err != nil {
    return err
}
```

Affected commands: `apply`, `check`, `list`, `dag`, `bundle`, `package`, `compose`.

Note: `annotateKeyError` calls wrapping `resolveEnv` errors are unrelated and stay as-is.

## UX

**Prompt** (one `huh` input per missing key, empty rejected inline):

```
? env key "context" is not set *
> work

? env key "machine" is not set *
> laptop
```

**Persist hint** (stderr, printed immediately after prompt):

```
Hint: to persist, add to env.yaml:
  context: work
  machine: laptop
```

**Abort:** Ctrl+C during the prompt cancels the form and aborts the command cleanly.

## Testing

- **`CollectMissingKeys` unit tests** — nodes with missing keys, multiple distinct missing keys (dedup), overlapping keys across nodes, no missing keys, parse errors surfaced
- **`filterWithPrompt` non-TTY path** — pass `isTTY=false`, assert behavior identical to current `Filter` + `annotateKeyError`
- **`filterWithPrompt` TTY path with no missing keys** — pass `isTTY=true`, assert `CollectMissingKeys` short-circuits to `Filter` directly
- **TTY interactive path** — not unit-testable; verified by manual smoke test
