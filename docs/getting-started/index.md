# Installation

## install.sh (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs `dotd` to `~/.local/bin`. Requires only `curl`.

**Install a specific version:**

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --version v0.6.0
```

**Install to a custom directory:**

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --dir /usr/local/bin
```

## From source

```sh
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotd
```

Requires Go 1.24 or later.

---

## Quick start

If you already have a dotfiles repo, point dotd at it and run:

```sh
# Apply everything — symlinks, packages, init.sh
dotd apply -f ~/dotfiles

# See what would change without touching anything
dotd apply -f ~/dotfiles --dry-run

# Check current state across all stages
dotd check -f ~/dotfiles
```

`dotd setup` and `dotd init` wire `init.sh` into your shell for you. If you ran a
bare `dotd apply` without them, add the source line once yourself:

```sh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc   # zsh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.bashrc  # bash
```

If you're starting from scratch, use `dotd setup` to scaffold the repo structure and wire up your shell automatically:

```sh
dotd setup
```

See [Your first machine](first-machine.md) for a step-by-step walkthrough.

---

## Dotfiles repo layout

```
dotfiles/
  shellrc/          ← shell scripts sourced into init.sh, in dependency order
  config/           ← config files symlinked under $config (default ~/.config)
  bin/              ← executables symlinked into $bin (default ~/.local/bin/dot-dagger, on $PATH)
  env.yaml          ← your environment context (os, shell, context, etc.)
  packages.yaml     ← package registry
  .dagger           ← per-directory config for files that can't carry annotations
```

Any file in `shellrc/`, `config/`, or `bin/` is picked up automatically. [Annotations](../concepts/annotations.md) — comments at the top of each file — control conditions, load order, and package requirements.

### Naming conventions

**`dot-` prefix** is stripped when computing symlink destinations, so `dot-` prefixed names become hidden paths:

```
config/dot-config/nvim/init.lua  →  ~/.config/nvim/init.lua
config/nvim/init.lua             →  ~/.config/nvim/init.lua
```

**`nosync-` prefix** is stripped from path components when computing a file's identity. Useful for machine-specific directories you don't want to commit:

```
nosync-work/shellrc/aliases.sh  →  identity: work.shellrc.aliases
```

See [File identity](../concepts/file-identity.md) for the full rules.
