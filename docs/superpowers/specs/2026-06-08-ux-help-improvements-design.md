# Design: UX and Help System Improvements

**Date:** 2026-06-08
**Status:** Approved

## Problem

Several friction points discovered during real usage:

1. `@when` expression syntax is undocumented at the point of use — users guess comma-separated and hit parse errors
2. `dotd env show` shows evaluated values with no indication of which are shell expressions
3. `dotd env set` gives no hint that shell expressions need single-quote escaping
4. No way to find where env.yaml lives from the CLI
5. Failed commands print full usage before the error — error is buried
6. `--debug` is not a flag; `--log-level debug` is non-obvious
7. No conceptual reference — users must infer mental model from command names alone

## Changes

### 1. `@when` description

`WhenType.Description()` in `internal/annotation/registry.go` updated to return a multi-line syntax block:

```
Condition for when this file is active.

  key=value              single condition       os=macos
  key=v1,v2             match any value        os=macos,linux
  expr AND expr         both must match        os=macos AND context=work
  expr OR expr          either matches         os=macos OR os=linux
  (expr)                grouping               (os=macos OR os=linux) AND context=work

Comma separates multiple values for ONE key. Use AND to join two conditions.
```

`annotate_cmd.go` currently concatenates `t.Description() + "  (clear the field to remove)"` and passes the result inline to huh. A multi-line description breaks this concatenation. Fix: for `KindText` fields, print `t.Description()` as a preamble via `fmt.Fprintln` before invoking the huh prompt, and pass only `"(enter value, or clear to remove)"` as the huh description. This applies to all `KindText` fields — other KindText types have short descriptions so the change is invisible to them.

`WhenType.Validate` error updated to:
```
@when: expected format key=value or key=v1,v2 (got %q)
hint: use AND/OR to join conditions; comma separates multiple values for one key (e.g. os=macos,linux)
```

### 2. `dotd env show` — expression annotation

`newEnvShowCmd` loads the raw env.yaml map alongside the resolved map. For each key, if the raw value matches the `$(...)` shell expression pattern, the expression is printed in brackets after the resolved value:

```
context=work
hostname=personal-mac.local	[$(hostname)]
os=macos	[$(dotd get-os)]
```

Alignment: a single `\t` separates the `key=value` portion from the `[$(expr)]` annotation. No dynamic padding — consistent, simple, terminal-width-independent.

Values from DOTD_* shell vars or `--env` flags show as plain values with no annotation. The existing `diff` command covers the source-attribution question.

Implementation: call `env.Load(cfg.envFile)` inside `newEnvShowCmd.RunE` to get the raw map. Detect shell expressions inline: `strings.HasPrefix(v, "$(") && strings.HasSuffix(v, ")")`. No export needed — the check is two lines and doesn't belong in the internal API.

### 3. `dotd env set` — shell expression hint

`newEnvSetCmd` gets a `Long` description:

```
Set a key in env.yaml.

To store a shell expression that evaluates at runtime, use single quotes
to prevent the shell from expanding it:

  dotd env set os '$(dotd get-os)'
  dotd env set hostname '$(hostname)'

Values stored as $(…) are evaluated each time dotd runs.
```

### 4. `dotd env path`

New subcommand `dotd env path` added to `newEnvCmd`. Prints `cfg.envFile` and exits.

Short description: `Show the path to env.yaml`

### 5. Error output — silence usage on domain errors

`SilenceUsage: true` added to the root command struct literal in `newRootCmd` (alongside the existing `SilenceErrors: true`). This suppresses the usage block for all errors — domain errors, arg-count errors, and unknown-flag errors alike. The error message itself is always sufficient to understand what went wrong. `SilenceErrors` is already set so errors are never printed twice.

### 6. `--debug` flag

A `debug bool` field added to `appConfig`. A `--debug` bool persistent flag bound to `cfg.debug` added to the root command alongside `--log-level`. In `configureLogger`:

- `--debug` sets the effective log level to `"debug"` as a baseline
- `--log-level` is applied afterward and wins if explicitly set
- So `--debug` alone → debug; `--debug --log-level info` → info; neither → default

### 7. `dotd concepts`

New command `dotd concepts` in the "Additional Commands" group. Prints a static multi-section reference to stdout.

Sections:
- **Pipeline** — walk → filter → order → act → init.sh, one sentence each
- **`@when` predicates** — full expression syntax with examples (same content as the updated description, expanded)
- **Annotations** — all keys with one-line descriptions: `@when`, `@action`, `@after`, `@name`, `@require`, `@request`, `@disable`
- **env.yaml** — format (flat key: value YAML), shell expressions, resolution order: env.yaml is baseline, DOTD_* shell vars override it, `--env` flags override everything
- **Directory naming** — `dot-` prefix, `nosync-` prefix, `.d` suffix for compose targets

Content is a Go string constant. No external files, no i18n.

**TODO:** Add sub-topic routing (`dotd concepts when`, `dotd concepts env`) once the flat version is validated with users.

## Files Changed

| File | Change |
|------|--------|
| `internal/annotation/registry.go` | Update `WhenType.Description()` and `WhenType.Validate()` error |
| `cmd/dotd/env.go` | Update `newEnvShowCmd`, `newEnvSetCmd Long`, add `newEnvPathCmd` |
| `cmd/dotd/main.go` | Add `--debug` flag, set `SilenceUsage = true` |
| `cmd/dotd/concepts_cmd.go` | New file: `newConceptsCmd` |
| `.claude/TODO.md` | Add: expand concepts to sub-topics |

## Testing

- `TestEnvShowExprAnnotation` — show output includes `[$(expr)]` for shell-expression values; plain values have no annotation
- `TestEnvPathCmd` — prints resolved env file path
- `TestDebugFlagSetsLogLevel` — `--debug` produces debug-level log output
- `TestWhenValidate_errorHint` — existing `WhenType.Validate` test updated to assert new error message (the error text is changing; this test will fail until updated)
- `SilenceUsage` and `concepts` verified manually (output content, no errors)
