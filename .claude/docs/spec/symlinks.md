# §9, §10 — Symlink Strategy & Drift Detection

## 9. Symlink Strategy

### `dots/` → `$HOME`

```
dots/dot-zshrc                     → ~/.zshrc
dots/dot-config/tmux/tmux.conf     → ~/.config/tmux/tmux.conf
dots/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
```

Every `dot-` prefix at every path level is replaced with `.`. Non-`dot-` components are used as-is.

### `bin/` → managed bin dir

```
bin/tmux-sessionizer → ~/.local/bin/dot-dagger/tmux-sessionizer
```

The managed bin dir is added to PATH in the generated `init.sh`. It is the only PATH addition dot-dagger makes.

### `@symlink` → explicit destination

Any file anywhere in the repo can declare `@symlink <path>` to be symlinked to an explicit destination. Tilde (`~/`) is expanded to the home directory.

### Ownership

A symlink is owned by dot-dagger if its current target starts with the repo root path. Owned symlinks are updated freely (e.g. when switching between variant files). Foreign symlinks — pointing outside the repo — require `--force`.

### Conflict handling

If a real file exists where dot-dagger expects one of its symlinks, the tool warns and requires `--force` to proceed. It never silently removes or overwrites files it does not own.

### Deactivation cleanup

When a file's predicate no longer matches, `dotd apply` removes its deployed artifacts. Default behaviour for v1 — configurable later via `on_deactivate` setting.

---

## 10. Drift Detection

`dotd status files` compares deployed state to source at runtime. No state file — candidate paths are derived from active nodes in the current environment.

Symlink states reported:

| State | Meaning |
|-------|---------|
| OK | Symlink exists and points to the correct source |
| Missing | Nothing at the expected destination |
| WrongTarget | Symlink exists but points elsewhere |
| Conflict | Real file at the expected destination |

Fully managed directories get a full recursive diff. Partially managed directories (like `~/`) only diff managed files — unmanaged siblings are ignored.

`dotd status env` checks that builtins are detected, context is set, the rc file is wired, and the repo is not behind origin.
