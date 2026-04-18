# §7 — Config Files

## `env.yaml` (`~/.config/dot-dagger/env.yaml`, not committed — machine local)

Stores the resolved environment for this machine. The dotfiles repo location is also read from here, so `dotd` does not need to be run from within the dotfiles repo.

```yaml
env:
  context: work
  role: desktop

dotfiles_repo: ~/dotfiles
```

All fields are optional. Keys under `env` override auto-detected values. `dotfiles_repo` sets the path to the dotfiles repo for this machine.

---

## `.dotd.yaml` (per-directory, in dotfiles repo)

The only config file in the dotfiles repo. See [dag.md §6](dag.md) for full structure.

There is no separate `config.yaml` or global schema file — environment keys, annotation handlers, and convention directory names are not configurable at the repo level in the current implementation.

---

See [predicates.md](predicates.md) for env key definitions, resolution precedence, and handling of missing keys.
