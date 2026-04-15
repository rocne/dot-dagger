# dotr

Composable shell environment management. Write annotations in your dotfiles, run `dotr apply`, and your `init.sh`, symlinks, and packages stay in sync across machines.

Each tool handles one concern and works standalone. `dotr` composes them into a single pass.

---

## Install

```sh
# Build from source (all tools)
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotr ./cmd/dotd ./cmd/dotl ./cmd/dotp ./cmd/dote
```

Each tool also [releases independently](../../releases) as a pre-built binary for macOS and Linux (amd64/arm64).

---

## Quick start

```sh
# 1. Apply everything — symlinks, packages, init.sh
dotr apply --files ~/dotfiles

# 2. Wire init.sh into your shell rc
echo 'source ~/.config/dot-dagger/init.sh' >> ~/.zshrc
```

After that, re-run `dotr apply` whenever your dotfiles change.

---

## Dotfiles repo layout

```
dotfiles/
  scripts/        ← shell scripts sourced into init.sh, in DAG order
  conf/           ← config files symlinked into $HOME
  bin/            ← executables symlinked onto $PATH
  env.yaml        ← your environment context (os, shell, context, etc.)
  packages.yaml   ← package registry
  .dotr.yaml      ← metadata for files that can't carry annotations
```

`dot-` prefix is stripped from directory names in symlink destinations:
`conf/dot-config/nvim/init.lua` → `~/.config/nvim/init.lua`

`nosync-` prefix is stripped from logical names (useful for machine-specific dirs you don't push):
`nosync-work/scripts/aliases.sh` → logical name `work.scripts.aliases`

---

## Tools

### dotr — orchestrator

Runs a full reconciliation pass in one command: resolve environment → evaluate predicates → install packages → apply symlinks → generate `init.sh`.

```sh
dotr apply --files ~/dotfiles          # full reconciliation
dotr apply --files ~/dotfiles --dry-run  # print what would happen
dotr check --files ~/dotfiles          # validate all stages, no changes
```

`dotr apply` is equivalent to running `dote`, `dotd`, `dotl`, and `dotp` in sequence with the same `--files` argument. Any tool can be used standalone or scripted independently — `dotr` is a convenience, not a gate.

---

### dote — environment

Detects your environment and resolves the `Env` map used by all predicate evaluation. Owns `env.yaml`.

**Auto-detected keys:**

| Key | Examples |
|-----|---------|
| `os` | `macos`, `linux` |
| `distro` | `ubuntu`, `fedora`, `sequoia` |
| `shell` | `zsh`, `bash`, `fish` |

**Custom keys** are declared in `env.yaml` and not auto-detected. Useful for things like `context`:

```yaml
# env.yaml
env:
  context: personal
```

```sh
dote show                      # print the fully resolved environment
dote show --env context=work   # override a key for debugging
```

`dote show` is the go-to for diagnosing why a predicate isn't matching.

---

### dotd — init.sh generation

Walks `scripts/`, evaluates `@when` predicates, resolves `@after` ordering into a DAG, and writes a single `init.sh` you source once from your shell rc.

**Owns:** `scripts/`

```sh
dotd apply --files ~/dotfiles                       # generate init.sh
dotd apply --files ~/dotfiles --dry-run             # print what would be written
dotd check --files ~/dotfiles                       # validate DAG, report status
dotd apply --files ~/dotfiles --init-file ~/init.sh # custom output path
```

Scripts are sourced in topological order. Files with no `@after` are ordered alphabetically by logical name within each frontier — output is always deterministic.

---

### dotl — symlink management

Walks `conf/` and `bin/`, plans symlinks into `$HOME` (and `$PATH`), and applies them. Detects drift — missing, wrong-target, and conflicting symlinks are reported.

**Owns:** `conf/`, `bin/`

```sh
dotl apply --files ~/dotfiles    # plan and apply all symlinks
dotl check --files ~/dotfiles    # report state without changes
dotl remove --files ~/dotfiles   # remove all owned symlinks
```

`conf/` files are symlinked relative to `$HOME`. `dot-` prefix stripped from path components:
`conf/dot-config/nvim/init.lua` → `~/.config/nvim/init.lua`

Override the destination root for a subtree via `.dotr.yaml`:

```yaml
# conf/dot-config/nvim/.dotr.yaml
directory:
  link_root: ~/.config/nvim
```

Standalone `dotl` is unconditional — all files in `conf/` and `bin/` are linked regardless of `@when`. Use `dotr` for predicate-filtered linking.

---

### dotp — package management

Reads `@require` and `@request` annotations across your dotfiles and installs packages using whichever package manager is available. Owns `packages.yaml`.

**Owns:** `packages.yaml`

```sh
dotp install --files ~/dotfiles   # install all declared packages
dotp check --files ~/dotfiles     # report status without installing
dotp list --files ~/dotfiles      # list all declared packages
dotp install --files ~/dotfiles --dry-run  # print what would run
```

Annotations used in any file in your dotfiles repo:

```sh
# @require ripgrep   — hard gate: file only active if ripgrep is installed or installable
# @request fzf       — soft ask: file always active; fzf installed if possible, skipped if not
```

Package manager priority is set in `.dotr.yaml` at your repo root:

```yaml
# .dotr.yaml
dote:
  package_managers:
    priority: [brew, apt, dnf, pacman, pip, cargo]
```

Standalone `dotp` is unconditional (no predicate filtering). Use `dotr` for predicate-scoped package installs.

---

## Annotations

Annotations go at the top of a file in comments. Shell: `#`, C-style: `//`. Scanning stops at the first blank or non-comment line.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep

# script body starts here
```

### `@when` — inclusion predicate

A file with no `@when` is always active. Multiple `@when` lines are ANDed together.

```sh
# @when os=macos,linux        # os is macos OR linux (comma = same-key OR)
# @when context=work          # ANDed with the line above
# effective: (os=macos OR os=linux) AND context=work
```

Full grammar: `AND` binds tighter than `OR`, parentheses override, comma is same-key OR shorthand.

```sh
# @when os=macos OR (shell=zsh AND exists(brew))
```

Built-in predicate functions:

| Function | True when |
|----------|-----------|
| `exists(binary)` | binary is on PATH |
| `installed(pkg)` | package binary is present on PATH |
| `installable(pkg)` | package has a registry entry and an available manager |

### `@after` — DAG ordering

```sh
# @after scripts/base/             # all active files under scripts/base/
# @after tmux.scripts.helpers      # specific file by logical name
```

If referenced files aren't active, the dependency is a no-op — never an error.

### `@name` — logical name override

Used for variant files that represent the same logical unit under different conditions. Exactly one variant must be active at a time.

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

### `@symlink` — explicit symlink destination

Usually unnecessary — `conf/` handles the common case. Use when a file outside `conf/` needs symlinking, or to override the conventional destination.

```sh
# @symlink ~/.gitconfig
```

### `@retain-prefix` — opt out of `dot-` stripping

```sh
# @retain-prefix
# conf/dot-tmux.conf → ~/.dot-tmux.conf (prefix preserved)
```

---

## packages.yaml

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
    binary: rg       # binary name differs from package name
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

  no-binary-lib:
    check: "python3 -c 'import somelib'"   # custom existence check
    pip:
      package: somelib
```

- Logical name = package name = binary name unless overridden
- `check` defaults to `which {binary}`
- `{package}` and `{binary}` are substituted at runtime

---

## .dotr.yaml

Per-directory metadata for files that can't carry annotations (JSON, XML, binary). Can appear in any directory; `defaults` cascades to subdirectories.

```yaml
directory:
  when: "os=macos"       # gates traversal — directory skipped if false
  link_root: ~/.config/nvim  # symlink root override for this subtree

defaults:
  when: "context=work"   # ANDed with each file's own @when

files:
  - path: dot-gitconfig-work
    when: "context=work"
    symlink: ~/.gitconfig

  - path: dot-gitconfig-personal
    when: "context=personal"
    symlink: ~/.gitconfig
```

---

## License

MIT
