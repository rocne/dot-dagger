# §4 — The Predicate System

Every file can declare a `when` condition. A file is active if its effective predicate evaluates to true against the current environment.

A directory-level `when` in `.dot-dagger.yaml` applies to all files in that directory and all subdirectories as a shared default. The effective predicate for any file is:

```
directory_when AND file_when
```

A file with no `when` is implicitly true — active whenever its directory predicate is satisfied.

---

## Environment keys

| Key | Auto-detected? | Method | Examples |
|-----|---------------|--------|---------|
| `os` | Yes | `runtime.GOOS`, normalized | `macos`, `linux` |
| `distro` | Yes | `/etc/os-release` or `sw_vers` | `ubuntu`, `sequoia` |
| `shell` | Yes | `$SHELL`, basename, lowercased | `zsh`, `bash` |
| `context` | No | Must be set explicitly | `personal`, `work` |

`darwin` and `macos` are treated as aliases — `runtime.GOOS` returns `darwin`, which is normalized to `macos` at detection time.

Custom keys are declared in `config.yaml` with optional `detect`, `cmd`, `default`, and `values` fields.

---

## Environment resolution precedence

1. `--env key=val` CLI flag
2. Explicit value in `env.yaml`
3. `cmd` output from `config.yaml`
4. `detect` auto-detection
5. `default` static fallback from `config.yaml`
6. Unset — surfaces as error or prompt, never silent

### Missing keys at apply time

If `context` or any other required key is unset when `dotd apply` runs, the tool prompts interactively if a TTY is present, or halts with a clear error in non-interactive mode. Files gated on unset keys are not silently excluded.

`Resolve()` never prompts — it returns `*MissingKeysError`. The CLI catches with `errors.As` and decides whether to prompt or halt based on TTY.

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
