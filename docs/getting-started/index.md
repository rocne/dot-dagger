# Installation

## install.sh (recommended)

```sh
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs `dotr` to `~/.local/bin`.

Requires the [GitHub CLI](https://cli.github.com) authenticated with `gh auth login`. This requirement will be removed when the repository goes public.

**Install a specific tool:**

```sh
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- dotl
```

**Install a specific version:**

```sh
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- dotr --version v0.1.4
```

**Install to a custom directory:**

```sh
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --dir /usr/local/bin
```

## From source

```sh
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotr ./cmd/dotd ./cmd/dotl ./cmd/dotp ./cmd/dote
```

Requires Go 1.24 or later.

---

## Quick start

If you already have a dotfiles repo, point dotr at it and run:

```sh
# Apply everything — symlinks, packages, init.sh
dotr apply -f ~/dotfiles

# Wire init.sh into your shell
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc   # zsh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.bashrc  # bash

# See what would change without touching anything
dotr apply -f ~/dotfiles --dry-run

# Check current state across all stages
dotr check -f ~/dotfiles
```

If you're starting from scratch, use `dotr setup` to scaffold the repo structure and wire up your shell automatically:

```sh
dotr setup
```

See [Your first machine](first-machine.md) for a step-by-step walkthrough.

---

## Dotfiles repo layout

```
dotfiles/
  scripts/          ← shell scripts sourced into init.sh, in dependency order
  conf/             ← config files symlinked into $HOME
  bin/              ← executables symlinked onto $PATH
  env.yaml          ← your environment context (os, shell, context, etc.)
  packages.yaml     ← package registry
  .dot-dagger.yaml  ← per-directory config for files that can't carry annotations
```

Any file in `scripts/`, `conf/`, or `bin/` is picked up automatically. [Annotations](../concepts/annotations.md) — comments at the top of each file — control conditions, load order, and package requirements.

### Naming conventions

**`dot-` prefix** is stripped when computing symlink destinations, so files named with a `dot-` prefix become hidden files in `$HOME`:

```
conf/dot-zshrc                 →  ~/.zshrc
conf/dot-config/nvim/init.lua  →  ~/.config/nvim/init.lua
```

**`nosync-` prefix** is stripped from path components when computing a file's identity. Useful for machine-specific directories you don't want to commit:

```
nosync-work/scripts/aliases.sh  →  identity: work.scripts.aliases
```

See [File identity](../concepts/file-identity.md) for the full rules.
