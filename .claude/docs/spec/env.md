# §7 — Config Files

## `config.yaml` (committed — repo root)

Declares the environment schema and annotation handlers.

```yaml
env:
  os:     { detect: true }
  distro: { detect: true }
  shell:  { detect: true }
  context:
    detect: false
    values: [personal, work]
  role:
    default: desktop
  host:
    cmd: hostname

annotation_handlers:
  requires: dag-pkg procure
```

---

## `~/.config/dot-dagger/env.yaml` (not committed — machine local)

```yaml
env:
  context: work
  role: desktop
```

---

See [predicates.md](predicates.md) for env key definitions, resolution precedence, and handling of missing keys.
