# §1–2 Overview & Directory Conventions

## 1. Overview

dot-dagger is a dotfiles composition engine. It walks a dotfiles directory tree, evaluates predicate expressions on each file, resolves a DAG-based sourcing order, generates a single shell init file, and symlinks dotfiles and bin commands into place.

It is intentionally minimal. Package management is a separate tool. The predicate system is the core engine — everything else is convention.

---

## 2. Directory Conventions

Any directory in the dotfiles repo can contain these conventionally-named subdirectories. Conventions apply at any depth.

| Directory | Behavior |
|-----------|----------|
| `scripts/` | Files are sourced into the generated `init.sh` in DAG order |
| `bin/` | Files are symlinked into the managed bin dir |
| `dots/` | Files are symlinked into `$HOME` with `dot-` prefix transformed |

These conventions require zero configuration.

### The `dot-` prefix

Files in `dots/` use `dot-` instead of `.` so they remain visible in the repo. When computing **symlink destinations**, every path component under `dots/` that starts with `dot-` has the prefix replaced with `.`:

```
dots/dot-zshrc                     → ~/.zshrc
dots/dot-config/tmux/tmux.conf     → ~/.config/tmux/tmux.conf
dots/dot-config/dot-tmux/tmux.conf → ~/.config/.tmux/tmux.conf
```

Path components without the `dot-` prefix are used as-is. Replacement applies at every level of the path, not just the top level.

**Important:** This is distinct from logical name derivation (see [dag.md](dag.md)), where `dot-` is simply stripped entirely. The two transformations serve different purposes:

| Context | Rule | Example |
|---------|------|---------|
| Logical names (DAG) | strip `dot-` entirely | `dot-zshrc` → `zshrc` |
| Symlink destinations | replace `dot-` with `.` | `dot-zshrc` → `.zshrc` |

To opt out of prefix transformation for a specific file, use `@retain-prefix` in the file's annotation block, or declare `retain_prefix: true` in `.dotd.yaml`. The `RetainPrefix` flag applies only to the file's own path component; intermediate directory components are always transformed.

### The `nosync-` prefix

Any file or directory prefixed with `nosync-` is automatically gitignored, fully functional at runtime, and has the prefix stripped from its logical name and symlink destination. Applies at any level.

During `dotd install`, the tool ensures `nosync-*` is present in `.gitignore` before any other operation. This prevents accidental staging of private files on fresh repos.
