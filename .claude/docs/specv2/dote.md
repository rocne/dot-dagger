# dote — Environment Resolution

`dote` resolves the runtime environment used by all suite tools for predicate evaluation. It owns `env.yaml` and provides the `Env` map that `dota` evaluates predicates against.

---

## Responsibilities

1. **`env.yaml` loading** — reads and validates the environment config file
2. **Built-in detection** — OS, distro, shell (carries forward from v1 `spec/env.md`)
3. **Custom detector hooks** — users and tools register additional detectors
4. **`Env` map output** — resolved key/value map passed to `dota` for predicate evaluation

---

## CLI

`dote` has a CLI primarily for debugging:

| Command | Description |
|---------|-------------|
| `dote show` | Dump the fully resolved `Env` map |

Useful for diagnosing why a predicate isn't behaving as expected.

---

## Custom Detector Hooks

`dote` supports registered detectors for custom env keys, beyond the built-in OS/distro/shell detection. Tools and users can extend detection without modifying `dote`.

```go
dote.RegisterDetector("editor", myEditorDetector)
```

Detectors receive no arguments and return `(string, error)`. The returned string becomes the value for their key in the `Env` map.

---

## Missing Keys

Carries forward from v1: missing required keys prompt (TTY) or halt (non-interactive). `dote` returns a `*MissingKeysError`; the CLI catches it and decides based on TTY. Never silently excludes files due to unset keys.

---

## `.dotr.yaml` integration

`dote` reads the `dote:` section of `.dotr.yaml` for per-directory environment overrides. Details TBD — deferred until implementation begins.
