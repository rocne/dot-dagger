# dotd

Shell init generation tool. Walks `scripts/`, evaluates conditions, resolves the dependency graph, and writes `init.sh`.

Unlike `dotr dag`, `dotd` operates **without condition filtering** — all files are included regardless of `@when`. Use `dotd` for unconditional introspection or scripting. Use `dotr dag` when you want conditions applied.

## Commands

### dotd apply

Resolve load order and write `init.sh`.

```sh
dotd apply -f ~/dotfiles
dotd apply -f ~/dotfiles --dry-run
dotd apply -f ~/dotfiles --init-file ~/init.sh
dotd apply -f ~/dotfiles --bin-dir ~/.local/bin
```

### dotd check

Validate without writing `init.sh`.

```sh
dotd check -f ~/dotfiles
dotd check -f ~/dotfiles --verbose   # show numbered load order
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `~/dotfiles` | Path to dotfiles repo |
| `--env-file` | `~/dotfiles/env.yaml` | Path to env.yaml |
| `--init-file` | `~/.local/share/dot-dagger/init.sh` | Path to write init.sh |
| `--bin-dir` | `~/.local/bin` | Bin directory (added to PATH in init.sh) |
| `--dry-run` | false | Print actions without executing |
| `--verbose` | false | Detailed output |

## What init.sh contains

The generated `init.sh` is a shell script that sources all active scripts in dependency order. It's designed to be sourced from your shell's startup file (`.zshrc`, `.bashrc`, etc.):

```sh
source ~/.local/share/dot-dagger/init.sh
```

The file itself contains only `source` calls — no conditions, no loops. All condition evaluation happened at `apply` time.
