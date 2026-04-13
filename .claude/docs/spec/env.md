# §7 — Config Files

## `config.yaml` (committed — dotfiles repo root)

Declares the environment schema, annotation handlers, and configurable tool settings.

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

# Optional overrides for convention directory names (power user)
dirs:
  scripts: scripts
  bin: bin
  conf: conf

# Optional override for managed bin dir
bin_dir: ~/.local/bin/dot-dagger
```

The `bin_dir` defaults to `~/.local/bin/dot-dagger/` (or `$XDG_BIN_HOME/dot-dagger/` if `$XDG_BIN_HOME` is set).

---

## `~/.config/dot-dagger/env.yaml` (not committed — machine local)

Stores the resolved environment for this machine. The dotfiles repo location is also read from here, so `dotd` does not need to be run from within the dotfiles repo.

```yaml
env:
  context: work
  role: desktop

dotfiles_repo: ~/dotfiles
```

---

See [predicates.md](predicates.md) for env key definitions, resolution precedence, and handling of missing keys.
