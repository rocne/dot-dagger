# File identity

Every file in your dotfiles repo has a **logical name** — a stable identifier derived from its path. Logical names are used for dependency declarations (`@after`), variant files (`@name`), and anywhere dotd needs to refer to a file independent of its exact location.

## How logical names are derived

The logical name is computed from the file's path relative to the repo root:

1. Split the path into components
2. Strip `nosync-` and `dot-` prefixes from each component
3. Strip the file extension from the last component
4. Join with `.`

```
scripts/helpers.sh              →  scripts.helpers
conf/dot-zshrc                  →  conf.zshrc
conf/dot-config/nvim/init.lua   →  conf.config.nvim.init
nosync-work/scripts/work.sh     →  work.scripts.work
bin/my-tool                     →  bin.my-tool
```

## The dot- prefix

The `dot-` prefix is a naming convention for files that should become hidden files when symlinked. Unix hidden files start with `.`, but a dotfiles repo full of files starting with `.` is awkward to navigate.

The convention is to name them with `dot-` instead:

```
conf/dot-zshrc     →  symlinked to ~/.zshrc
conf/dot-gitconfig →  symlinked to ~/.gitconfig
conf/dot-config/   →  symlinked under ~/.config/
```

The `dot-` prefix is stripped when computing both the logical name and the symlink destination.

### Keeping the prefix with @retain-prefix

If you actually want a file to remain named with `dot-` or `nosync-` in its destination, use `@retain-prefix`:

```sh
# conf/dot-tmux.conf
# @retain-prefix
# → symlinked to ~/.dot-tmux.conf  (prefix kept)
```

Without `@retain-prefix`:
```
conf/dot-tmux.conf  →  ~/.tmux.conf
```

With `@retain-prefix`:
```
conf/dot-tmux.conf  →  ~/.dot-tmux.conf
```

This applies to both `dot-` and `nosync-` on the filename. Directory components above the filename are always stripped regardless.

## The nosync- prefix

The `nosync-` prefix marks directories that you don't want to commit to your dotfiles repo — machine-specific configs, work credentials, or anything else that shouldn't be shared. The prefix is stripped from logical names so files inside still get meaningful identities:

```
nosync-work/scripts/vpn.sh  →  work.scripts.vpn
```

You'd add `nosync-*/` to your `.gitignore` to keep these directories local.

## Overriding the logical name

Use `@name` to replace the derived logical name entirely. The main use case is **variant files** — two files that represent the same logical unit under mutually exclusive conditions:

```sh
# scripts/aliases-macos.sh
# @name scripts.aliases
# @when os=macos
```

```sh
# scripts/aliases-linux.sh
# @name scripts.aliases
# @when os=linux
```

Since only one is active at a time, they share the same logical name without conflict. Other files can `@after scripts.aliases` to depend on whichever variant is active.

Two active files with the same logical name is an error. Conditions on variant files must be mutually exclusive.
