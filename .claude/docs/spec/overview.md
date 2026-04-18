# §1–2 Overview & Directory Conventions

## 1. Overview

dot-dagger is a dotfiles composition engine. It walks a dotfiles directory tree, evaluates predicate expressions on each file, resolves a DAG-based sourcing order, generates a single shell init file, and symlinks config files into place.

It is intentionally minimal. Package management is a separate tool. The predicate system is the core engine — everything else is convention.

---

## 2. Directory Conventions

Any directory in the dotfiles repo can contain these conventionally-named subdirectories:

| Directory | Behavior |
|-----------|----------|
| `scripts/` | Files are sourced into the generated `init.sh` in DAG order |
| `bin/` | Files are symlinked into the managed bin dir and added to `PATH` |
| `conf/` | Files are symlinked to their destination path (default root: `~`) for third-party tools that expect config files at a fixed location |

These conventions require zero configuration. The key distinction: `scripts/` files are *sourced* into your shell; `conf/` files are *symlinked* into place for external tools.

### Where conventions apply

Special dirs are recognised anywhere in the dotfiles tree, as long as you have not already passed through a special dir to reach them. Once inside a special dir, further special dirs are not recognised — they are treated as regular directories.

```
scripts/           ✓  root-level
tmux/scripts/      ✓  topic-grouped
nosync-work/tmux/scripts/  ✓  private + topic-grouped
scripts/conf/      ✗  inside a special dir — ignored
scripts/scripts/   ✗  inside a special dir — ignored
```

A typical topic-grouped layout:

```
tmux/
  scripts/helpers.sh       → sourced
  bin/tmux-sessionizer     → symlinked to managed bin
  conf/dot-tmux.conf       → symlinked to ~/.tmux.conf
git/
  scripts/aliases.sh       → sourced
  conf/dot-gitconfig       → symlinked to ~/.gitconfig
```

### The `dot-` prefix

Files in `conf/` (and elsewhere) use `dot-` instead of `.` so they remain visible in the repo. When computing **symlink destinations**, every path component that starts with `dot-` has the prefix replaced with `.`:

```
conf/dot-zshrc                     → ~/.zshrc
conf/dot-config/tmux/tmux.conf     → ~/.config/tmux/tmux.conf
conf/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
```

The `dot-` transformation applies uniformly to every path component — files and directories alike. To opt out for a specific component, use `@retain-prefix` in the file's annotation block, or declare `retain_prefix: true` in `.dotd.yaml`. This applies at any level; there are no special cases for intermediate directories.

**Important:** This is distinct from logical name derivation (see [dag.md](dag.md)), where `dot-` is simply stripped entirely. The two transformations serve different purposes:

| Context | Rule | Example |
|---------|------|---------|
| Logical names (DAG) | strip `dot-` entirely | `dot-zshrc` → `zshrc` |
| Symlink destinations | replace `dot-` with `.` | `dot-zshrc` → `.zshrc` |

### The `nosync-` prefix

Any file or directory prefixed with `nosync-` has the prefix stripped from its logical name and symlink destination. It is the user's responsibility to add `nosync-*` to `.gitignore` to prevent accidental staging of private files. During `dotd setup` and `dotd check`, the tool will warn and offer to add `nosync-*` to `.gitignore` if it is missing — but will never do so silently.

```
nosync-work/conf/dot-gitconfig     → ~/.gitconfig  (nosync- stripped from destination)
nosync-work/scripts/aliases.sh     → sourced (nosync- stripped from logical name)
```

The stripping of `nosync-` from symlink destinations applies only to *implicit* destinations. If `@symlink` is declared explicitly, the destination is taken literally with no transformation. `@symlink` is the mechanism for overriding default destination behaviour.

### Convention names

`scripts/`, `bin/`, and `conf/` are fixed convention names — they are not configurable.
