# §9, §10 — Symlink Strategy & Drift Detection

## 9. Symlink Strategy

### `config/` → `~/.config` (default)

```
config/tmux/tmux.conf              → ~/.config/tmux/tmux.conf
config/dot-config/tmux/tmux.conf   → ~/.config/tmux/tmux.conf
config/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
nosync-work/config/dot-gitconfig   → ~/.gitconfig (with link_root: ~)
```

Every `dot-` prefix at every path level is replaced with `.`. Non-`dot-` components are used as-is. The `nosync-` prefix is stripped from implicit symlink destinations.

#### `link_root`

The default symlink destination root is `~`. Any directory's `.dagger` can override this with a `link_root` field under `directory:`:

```yaml
directory:
  link_root: ~/.config/someapp
```

All files under that directory that get symlinked will use `link_root` as the base instead of `~`. `link_root` cascades to subdirectories unless overridden by a closer `.dagger`.

### `bin/` → managed bin dir

```
bin/tmux-sessionizer → ~/.local/bin/dot-dagger/tmux-sessionizer
```

The managed bin dir defaults to `~/.local/bin/dot-dagger/` (or `$XDG_BIN_HOME/dot-dagger/` if `$XDG_BIN_HOME` is set). It is configurable in `config.yaml`. It is the only `PATH` addition dot-dagger makes, prepended in the generated `init.sh`.

### `@symlink` → explicit destination

Any file anywhere in the repo can declare `@symlink <path>` to be symlinked to an explicit destination. Destination path rules:

- Starts with `/` → absolute path
- Starts with `~/` → relative to `$HOME`
- Anything else → relative to `link_root` (or `~` if no `link_root` is set)

`@symlink` destinations are taken literally — no `nosync-` stripping, no `dot-` transformation. `@symlink` is the mechanism for overriding default destination behaviour.

### Ownership

A symlink is owned by dot-dagger if its current target starts with the dotfiles repo root path. Owned symlinks are updated freely (e.g. when switching between variant files). Foreign symlinks — pointing outside the repo — require `--force`.

### Conflict handling

If a real file exists where dot-dagger expects one of its symlinks, the tool warns and requires `--force` to proceed. It never silently removes or overwrites files it does not own.

### Deactivation cleanup

When a file's predicate no longer matches, `dotd apply` removes its deployed artifacts. Default behaviour for v1 — configurable later via `on_deactivate` setting.

---

## 10. Drift Detection

`dotd check` compares deployed state to source at runtime. No state file — candidate paths are derived from active nodes in the current environment.

Symlink states reported:

| State | Meaning |
|-------|---------|
| OK | Symlink exists and points to the correct source |
| Missing | Nothing at the expected destination |
| WrongTarget | Symlink exists but points elsewhere |
| Conflict | Real file at the expected destination |

Fully managed directories get a full recursive diff. Partially managed directories (like `~/`) only diff managed files — unmanaged siblings are ignored.
