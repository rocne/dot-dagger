# dotl

Symlink management tool. Walks `conf/` and `bin/`, plans symlinks into `$HOME` and `$PATH`, and applies them.

Unlike `dotr link`, `dotl` operates **without condition filtering**. Use `dotl` for unconditional introspection or scripting. Use `dotr link` when you want conditions applied.

## Commands

### dotl apply

Create or update symlinks.

```sh
dotl apply -f ~/dotfiles
dotl apply -f ~/dotfiles --dry-run
dotl apply -f ~/dotfiles --force       # overwrite conflicting files
dotl apply -f ~/dotfiles --link-root ~/custom-home
```

### dotl check

Report symlink state without making changes.

```sh
dotl check -f ~/dotfiles
dotl check -f ~/dotfiles --verbose    # include ok symlinks in output
```

States reported:

| State | Meaning |
|---|---|
| `ok` | Symlink exists and points to the right file |
| `missing` | Symlink doesn't exist yet |
| `wrong-target` | Symlink exists but points elsewhere |
| `conflict` | A non-symlink file exists at the destination |

### dotl remove

Remove symlinks owned by this dotfiles repo.

```sh
dotl remove -f ~/dotfiles
dotl remove -f ~/dotfiles --dry-run
```

Only symlinks pointing into the dotfiles repo are removed. Other files at destination paths are left alone.

## Flags

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `~/dotfiles` | Path to dotfiles repo |
| `--link-root` | `$HOME` | Symlink root for conf/ files |
| `--bin-dir` | `~/.local/bin` | Bin directory for bin/ files |
| `--dry-run` | false | Print actions without executing |
| `--force` | false | Overwrite conflicting (non-symlink) files |
| `--verbose` | false | Detailed output |

## How destinations are computed

For `conf/` files, the destination is computed by:

1. Taking the path relative to `conf/`
2. Stripping `dot-` and `nosync-` prefixes from each component
3. Joining with the `link_root` (default: `$HOME`)

```
conf/dot-zshrc              →  ~/.zshrc
conf/dot-config/nvim/init.lua  →  ~/.config/nvim/init.lua
```

For `bin/` files, the destination is `<bin-dir>/<filename>`.

## Overriding the symlink root

Use `.dot-dagger.yaml` in a subdirectory to override `link_root` for that subtree:

```yaml
# conf/dot-config/nvim/.dot-dagger.yaml
dotl:
  link_root: ~/.config/nvim
```

Or use `@symlink` on an individual file for a fully explicit path:

```sh
# @symlink ~/.gitconfig
```
