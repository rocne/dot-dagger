# Installation

Pick the channel for your platform. Homebrew and the apt/dnf repos install dot-dagger
as a managed package, so updates come through `brew upgrade` / `apt upgrade` /
`dnf upgrade`; `install.sh` is the dependency-free fallback.

!!! note "Package name vs. command"
    The **package** is `dot-dagger` (what you install); the **command** it
    installs is `dotd` (what you run). This holds on every channel.

## Homebrew (macOS / Linux)

```sh
brew install --cask rocne/tap/dot-dagger
```

Installs the `dotd` binary and tracks updates with `brew upgrade`.

## apt (Debian / Ubuntu)

The repo is **not** preconfigured — you must register it once with the setup
script (it adds the source and imports the signing key) **before** the install
will resolve:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.deb.sh' | sudo -E bash
sudo apt install dot-dagger
```

The setup script imports the repo signing key into a dedicated keyring
(`signed-by=`), so apt cryptographically verifies the index — no `[trusted=yes]`.

## dnf (Fedora / RHEL)

Same model — run the setup script first, then install:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.rpm.sh' | sudo -E bash
sudo dnf install dot-dagger
```

## install.sh

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs `dotd` to `~/.local/bin`. Requires only `curl`. Use this when you don't have Homebrew or the apt/dnf repo.

**Install a specific version:**

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --version v0.10.1
```

**Install to a custom directory:**

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --install-dir /usr/local/bin
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
