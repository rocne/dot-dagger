# dotr

A suite of composable shell environment management tools. Each tool handles one concern — environment, symlinks, packages, DAG init — and can be used standalone or composed via the `dotr` orchestrator.

## Tools

| Tool | Binary | Purpose |
|------|--------|---------|
| `dota` | library only | Annotation parsing and predicate engine — shared by all tools |
| `dote` | `dote` | Environment resolution — detects OS, distro, shell; loads `env.yaml` |
| `dotd` | `dotd` | DAG-ordered `init.sh` generation; owns `scripts/` |
| `dotl` | `dotl` | Symlink management; owns `conf/`, `bin/` |
| `dotp` | `dotp` | Package management; `@require`/`@request` annotations |
| `dotr` | `dotr` | Orchestrator — wires all tools into one pass |

---

## Install

### Download a release

Each tool releases independently. Download from the [releases page](../../releases), e.g.:

```sh
# macOS arm64, dote v0.1.0
curl -L https://github.com/rocne/dot-dagger/releases/download/dote%2Fv0.1.0/dote_v0.1.0_darwin_arm64.tar.gz | tar xz
sudo mv dote /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotr ./cmd/dotd ./cmd/dotl ./cmd/dotp ./cmd/dote
```

---

## Dotfiles repo layout

```
dotfiles/
  scripts/          ← shell scripts, sourced in DAG order into init.sh (owned by dotd)
  conf/             ← config files, symlinked into $HOME (owned by dotl)
  bin/              ← executables, symlinked onto $PATH (owned by dotl)
  packages.yaml     ← package registry (used by dotp)
  env.yaml          ← environment overrides (used by dote)
  .dotr.yaml        ← per-directory config for non-annotatable files
```

Directories prefixed with `dot-` have the prefix stripped in symlink destinations:
`conf/dot-config/nvim/init.lua` → symlinked to `~/.config/nvim/init.lua`.

Directories prefixed with `nosync-` have the prefix stripped from logical names:
`nosync-work/scripts/aliases.sh` → logical name `work.scripts.aliases`.

---

## Quick start with dotr

`dotr` runs a full reconciliation pass: resolve environment → build active file set → install packages → apply symlinks → generate `init.sh`.

```sh
# Full apply — run this whenever your dotfiles change
dotr apply --dotfiles ~/dotfiles

# Check state without making changes
dotr check --dotfiles ~/dotfiles

# Dry run — print what would happen
dotr apply --dotfiles ~/dotfiles --dry-run
```

Add to your shell rc:

```sh
# .zshrc / .bashrc
source ~/.config/dot-dagger/init.sh
```

---

## Standalone tools

Each tool can run independently. Standalone mode is unconditional — predicate gating (`@when`) requires `dotr` for the full evaluation pipeline.

### dote — environment

```sh
# Show the resolved environment (useful for debugging predicates)
dote show

# Override a key
dote show --env context=work
```

### dotd — init.sh generation

```sh
# Generate init.sh from scripts/ (with predicate evaluation)
dotd apply --dotfiles ~/dotfiles

# Check DAG and report script status
dotd check --dotfiles ~/dotfiles
```

### dotl — symlinks

```sh
# Apply all symlinks for conf/ and bin/
dotl apply --dotfiles ~/dotfiles

# Check symlink state (ok / missing / wrong-target / conflict)
dotl check --dotfiles ~/dotfiles

# Remove all owned symlinks
dotl remove --dotfiles ~/dotfiles
```

### dotp — packages

```sh
# Install all declared packages
dotp install --dotfiles ~/dotfiles

# Check package status without installing
dotp check --dotfiles ~/dotfiles

# List all declared packages
dotp list --dotfiles ~/dotfiles
```

---

## Annotations

Annotations appear in file comments at the top of a file. Shell files use `#`, C-style files use `//`.

```sh
#!/bin/bash
# @when os=macos
# @after scripts/base/
# Script body...
```

### Inclusion — `@when`

Gates whether a file is active. Multiple `@when` lines are ANDed.

```sh
# @when os=macos
# @when context=work
# effective: os=macos AND context=work
```

**Predicate grammar:**

```
expr   = and_expr (OR and_expr)*
       | atom (AND atom)*
       | "(" expr ")"
       | key=value[,value,...]
       | function(arg)
```

`AND` binds tighter than `OR`. Comma is same-key OR shorthand.

```sh
# @when os=macos,linux           # os is macos OR linux
# @when os=macos OR shell=zsh    # os is macos OR shell is zsh
# @when os=macos AND shell=zsh   # both must be true
```

**Built-in env keys:**

| Key | Auto-detected | Examples |
|-----|--------------|---------|
| `os` | Yes | `macos`, `linux` |
| `distro` | Yes | `ubuntu`, `fedora`, `sequoia` |
| `shell` | Yes | `zsh`, `bash`, `fish` |
| `context` | No — set in `env.yaml` | `work`, `personal` |

**Built-in predicate functions:**

| Function | Meaning |
|----------|---------|
| `exists(binary)` | True if binary is on PATH |
| `installed(pkg)` | True if package's binary is present on PATH |
| `installable(pkg)` | True if package has a registry entry and a manager available |

### Ordering — `@after`

Declares a DAG dependency. File is sourced after the referenced files in `init.sh`.

```sh
# @after scripts/base/              # all active files under scripts/base/
# @after tmux.scripts.helpers       # specific file by logical name
```

If no referenced files are active, the dependency is silently ignored.

### Identity — `@name`

Overrides the full logical name. Used for variant files — two files for the same logical unit under different conditions both declare the same `@name`. Exactly one must be active at a time.

```sh
# scripts/aliases-macos.sh
# @name scripts.aliases
# @when os=macos

# scripts/aliases-linux.sh
# @name scripts.aliases
# @when os=linux
```

### Symlinks — `@symlink`

Symlinks a file to an explicit destination. Usually unnecessary — `conf/` files are symlinked by convention.

```sh
# @symlink ~/.gitconfig
```

Relative paths resolve against the effective `link_root`.

### Retain prefix — `@retain-prefix`

Opts out of `dot-` stripping for the last path component in symlink destination resolution.

```sh
# @retain-prefix
# conf/dot-tmux.conf → symlinked to ~/.dot-tmux.conf (not ~/.tmux.conf)
```

### Package management — `@require` / `@request`

Registered by `dotp`. Available in any file.

```sh
# @require ripgrep    # hard gate — file only active if ripgrep is installed or installable
# @request fzf        # soft ask — file always active; fzf installed if possible
```

---

## env.yaml

Declares the environment context for your machine. Lives in your dotfiles repo root (or at `~/.config/dot-dagger/env.yaml`).

```yaml
env:
  context: personal
```

View the resolved environment at any time:

```sh
dote show
```

---

## packages.yaml

Registry of logical package names, package managers, and install commands. Lives at the dotfiles repo root.

```yaml
package_managers:
  brew:
    install: brew install {package}
  apt:
    install: apt install -y {package}
  dnf:
    install: dnf install -y {package}

packages:
  ripgrep:
    binary: rg        # binary name differs from package name
    brew: {}
    apt: {}
    dnf: {}

  fzf:
    brew: {}
    apt: {}

  some-tool:
    brew:
      install: brew tap someorg/sometool && brew install some-tool
    apt: {}
```

Package manager priority is set in `.dotr.yaml`:

```yaml
# .dotr.yaml
dote:
  package_managers:
    priority: [brew, apt, dnf, pacman, pip, cargo]
```

---

## .dotr.yaml

Per-directory config for files that cannot carry annotations (JSON, binary, XML). Can appear in any directory; settings cascade to subdirectories.

```yaml
# Gate traversal of this entire directory
directory:
  when: "os=macos"

# Default when for all files in this directory
defaults:
  when: "context=work"

# Per-file metadata
files:
  - path: dot-gitconfig-work
    when: "context=work"
    symlink: ~/.gitconfig

  - path: dot-gitconfig-personal
    when: "context=personal"
    symlink: ~/.gitconfig
```

Symlink destination for `conf/` subtrees can be overridden:

```yaml
# conf/dot-config/nvim/.dotr.yaml
directory:
  link_root: ~/.config/nvim
```

---

## Composability

`dotr apply` is equivalent to running:

```sh
dote show                          # verify env
dotd apply --dotfiles ~/dotfiles   # init.sh
dotl apply --dotfiles ~/dotfiles   # symlinks
dotp install --dotfiles ~/dotfiles # packages
```

Any tool can be used independently or scripted. `dotr` is a convenience, not a gate.

---

## License

MIT
