# dota — Annotation & Predicate System

`dota` is the shared annotation and predicate library for the dotr suite. All tools use it. It has no CLI. It has no dependencies within the suite — env context is passed in by the caller (provided by `dote`).

---

## Responsibilities

1. **Annotation parsing** — scan file comments for `@key value` annotations
2. **Predicate parsing and evaluation** — parse and evaluate `@when` expressions
3. **Custom annotation handlers** — tools register handlers for annotations they own
4. **Custom predicate functions** — tools register functions callable in `@when` expressions

---

## Built-in Predicate Functions

| Function | Description |
|----------|-------------|
| `exists(cmd)` | True if `cmd` is on PATH (`which`/`LookPath`) |

Env key checks (e.g. `os == "macos"`) are evaluated against the `Env` map provided by the caller. `dota` does not resolve env itself — that is `dote`'s responsibility.

---

## Extension Points

### Custom annotation handlers

A tool registers a handler for an annotation key it owns. When `dota` encounters that key during a scan, it dispatches to the registered handler.

```go
dota.RegisterAnnotation("package", dotp.HandlePackageAnnotation)
```

`--dry-run` suppresses handler side effects. Handlers must respect this.

### Custom predicate functions

A tool registers a named function that can be called inside any `@when` expression.

```go
dota.RegisterPredicate("installable", dotp.Installable)
```

Registered predicate functions receive the argument string and return `(bool, error)`.

---

## Unknown annotations and predicate functions

If `dota` encounters an annotation key or predicate function that is not registered, it emits an error or warning — **never silently ignores it or evaluates to false**. The behavior is configurable:

| Mode | Behavior |
|------|----------|
| `strict` (default) | Unknown annotation/predicate is an error; halts |
| `warn` | Logs a warning; continues |

This means running standalone `dotd` with `@package` annotations (a `dotp` concern) will warn or error unless `dotp` is registered or the mode is set to `warn`.

---

## Package annotations (owned by `dotp`)

`dotp` registers `@require`, `@request`, `installed()`, and `installable()`. See [dotp.md](dotp.md) for full details.

### `@require pkg` — hard gate

File is only active if `installable(pkg) || installed(pkg)`. Failure is loud (error).

### `@request pkg` — soft ask

File is always active. dotp installs if it can; silently skips if not.

Both solve the file-selection / package-installation ordering problem without circular dependencies — selection and installation intent are resolved in one pass.

---

## Annotation Syntax

Carries forward from v1 spec (see `spec/dag.md` §5 Annotations). Key rules:

- Annotations appear in file comments using the language's comment syntax
- Format: `@key value`
- Multiple annotations per file are allowed
- `@when` holds a predicate expression
- `@after`, `@name` defined in v1 carry forward, owned by `dotd`
- `@symlink`, `@retain-prefix` carry forward, owned by `dotl`
