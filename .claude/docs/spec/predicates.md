# §4 — The Predicate System

Every file can declare a `when` condition. A file is active if its effective predicate evaluates to true against the current environment.

A directory-level `when` in `.dagger` applies to all files in that directory and all subdirectories as a shared default. The effective predicate for any file is:

```
directory_when AND file_when
```

A file with no `when` is implicitly true — active whenever its directory predicate is satisfied.

---

## Environment keys

| Key | Source | Examples |
|-----|--------|---------|
| `os` | Shell expression in `env.yaml` — `$(dotd get-os)` | `macos`, `linux` |
| `distro` | Shell expression in `env.yaml` — `$(dotd get-hostname)` or manual | `ubuntu`, `fedora`, `macos` |
| `shell` | Shell expression in `env.yaml` — `$(basename $SHELL)` | `zsh`, `bash` |
| `context` | Explicit value in `env.yaml` | `personal`, `work` |

`dotd init` pre-populates `env.yaml` with shell expressions for `os`, `distro`, and `shell` so they work out of the box. `context` is left for the user to set.

`darwin` is normalised to `macos` by `dotd get-os`.

Additional env keys can be added to `env.yaml`. Any key present is available to predicates.

---

## Environment resolution precedence

1. `--env key=val` CLI flag (highest precedence)
2. Shell expression result from `env.yaml` (e.g. `os: $(dotd get-os)`)
3. Explicit value in `env.yaml`
4. Unset — halts with a clear error and hint; never silent

### Missing keys at apply time

If a required key is unset when `dotd apply` runs, the tool halts with an error and a hint (`"Hint: set it with --env or add it to env.yaml"`). Files gated on unset keys are never silently excluded.

---

## Predicate grammar

```
expr       = or_expr
or_expr    = and_expr (OR and_expr)*
and_expr   = atom (AND atom)*
atom       = "(" expr ")" | call | condition
call       = IDENT "(" IDENT ")"
condition  = KEY "=" VALUE ("," VALUE)*
```

`AND` binds tighter than `OR`. Parentheses override. Comma is same-key OR shorthand. `AND` and `OR` are case-sensitive uppercase keywords.

---

## Builtin predicate functions

| Function | Meaning |
|----------|---------|
| `exists(binary)` | True if binary is on PATH |

---

## Multi-line `@when`

Multiple `@when` lines are combined with AND:

```bash
# @when os=macos OR os=linux
# @when context=work
# effective: (os=macos OR os=linux) AND context=work
```
