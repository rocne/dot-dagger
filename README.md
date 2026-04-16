# dotr

A dotfiles management suite for people who use more than one machine.

If your setup is a single laptop running one OS and one shell, a handful of symlinks and a `.zshrc` is probably fine. But if you work across a personal Mac, a work Linux box, maybe a remote server — each with different shells, different package managers, different software installed — keeping one set of dotfiles that behaves correctly everywhere gets complicated fast.

The kind of problems dotr is designed for:

| | |
|---|---|
| **Multiple OS and shells** | Your Mac aliases and Homebrew paths don't belong on your Fedora box. Your zsh config and bash config share a lot but not everything. |
| **Work/personal coexistence** | Work aliases, internal hostnames, and credentials that can never go in a public repo — kept in a private or local-only directory, gated with `@when context=work`, coexisting cleanly with the rest of your dotfiles. |
| **Tool availability** | Scripts that use `fzf` or `ripgrep` shouldn't silently break on machines where those aren't installed. |
| **Package manager fragmentation** | Personal Mac uses Homebrew. Work Linux uses `apt` and you can't replace it. Remote server has `dnf`. One package name, right install command per machine. |
| **Script load order** | `00-base.zsh`, `10-path.zsh`, `20-tools.zsh` — hacking lexical sort into a load order. Breaks the moment you need to insert something, and says nothing about *why* the order matters. |

The conventional approaches work, up to a point.

For **conditions**, most setups reach for runtime `if` blocks in shell config:

```sh
if [ "$(uname)" = "Darwin" ]; then
  alias ls='ls -G'
  export HOMEBREW_PREFIX="/opt/homebrew"
fi

if [ "$CONTEXT" = "work" ]; then
  source ~/.work-aliases
fi

if command -v fzf &>/dev/null; then
  source ~/.fzf-bindings.zsh
fi
```

For **load order**, the standard move is numbered prefixes and split files:

```
conf/zsh/
  00-env.zsh
  10-path.zsh
  20-aliases.zsh
  30-tools.zsh
  31-fzf.zsh       ← had to squeeze this in between 30 and 40
  40-prompt.zsh
```

Both approaches have the same failure mode: they scale until they don't. The `if` blocks accumulate and nest. The numbers drift out of sync with the actual dependencies. You end up with a setup that works on your current machine but is hard to reason about, hard to extend, and brittle to move to a new one.

dotr's approach: **annotate files, not shell code**. Each file declares when it should be active. dotr evaluates those conditions once at apply time, builds the active file set for this machine, and writes a clean `init.sh` with no runtime branches.

```sh
#!/bin/bash
# @when os=macos
# @after scripts/base/

alias ls='ls -G'
export HOMEBREW_PREFIX="/opt/homebrew"
```

The same annotation system drives symlink management and package installation. One mental model, one place to declare intent.

---

## How dotr compares

| | [GNU Stow](https://www.gnu.org/software/stow/) + scripts | [dotbot](https://github.com/anishathalye/dotbot) | [chezmoi](https://www.chezmoi.io/) | dotr |
|---|---|---|---|---|
| Symlinks | ✓ | ✓ | ✗ (copies) | ✓ |
| Per-file conditions | ✗ | ✗ | Templates | Annotations |
| DAG-ordered init.sh | ✗ | ✗ | ✗ | ✓ |
| Package management | ✗ | Plugins | ✗ | ✓ (`@require`/`@request`) |
| Work/personal separation | Manual | Manual | Encryption | `@when context=work` |
| Multi-shell | Manual | Manual | Templates | `@when shell=zsh` |
| Central manifest | ✗ | ✓ | ✓ | ✗ (annotations in files) |
| Shell startup cost | Varies | Varies | Low | Low (conditions evaluated at apply time, not runtime) |

**Stow + scripts** is the baseline — symlinks work great, but conditions and ordering are all manual shell scripting. dotr is what you reach for when the scripts start to sprawl.

**dotbot** adds a structured YAML manifest for symlinks and actions. It's good at idempotent setup but has no concept of per-file conditions — you'd still write shell scripts or plugins for conditional behavior.

**chezmoi** is the most fully-featured alternative. It uses templates and copies files rather than symlinking, supports encryption for secrets, and has a large feature surface. If you need encrypted secrets in your dotfiles repo, chezmoi is probably the right choice. dotr's trade-off is simplicity: symlinks over copies, annotations over templates, no encryption.

The core dotr bet: **conditions belong on files, not in shell code or central manifests**. A file knows whether it applies to macOS. It knows it needs `ripgrep` installed. It knows it should be sourced after the base environment is set up. Keeping that knowledge with the file means you can look at any file and immediately understand when and how it's used — without cross-referencing a central config.

---

## Philosophy

**One annotation, one concern.** Each annotation does exactly one thing. `@when` controls inclusion. `@after` controls ordering. `@require` gates on a package. They compose but don't interfere.

**Convention over config.** Put files in `scripts/`, `conf/`, or `bin/` and they just work. Annotations and `.dot-dagger.yaml` are for exceptions, not the common case.

**Composable tools.** Every tool works standalone. `dotr` composes them, but you can script individual tools, use only the pieces you need, and understand the system by reading one piece at a time.

**Predicate evaluation, not runtime conditionals.** The active file set is resolved once at apply time. Your shell rc sources a pre-built `init.sh` with no branches — fast startup, predictable behavior.

---

## Install

### install.sh (recommended)

```sh
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs to `~/.local/bin`. Requires [gh CLI](https://cli.github.com) authenticated with `gh auth login`.

Install a specific tool or version:

```sh
# install dotl instead of dotr
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- dotl

# install a specific version
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- dotr --version v0.1.4

# install to a custom directory
curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
  https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --dir /usr/local/bin
```

### From source

```sh
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotr ./cmd/dotd ./cmd/dotl ./cmd/dotp ./cmd/dote
```

---

## How it works

1. **`dote`** detects your environment — OS, distro, shell — and loads any overrides from `env.yaml`. This produces the `Env` map used for all predicate evaluation.

2. **`dotd`** walks `scripts/`, evaluates `@when` predicates against the `Env` map, and resolves `@after` dependencies into a topological order. It writes a single `init.sh` that sources only the active scripts in the right order.

3. **`dotl`** walks `conf/` and `bin/`, plans symlinks into `$HOME` and `$PATH`, and applies them. It detects drift — missing, wrong-target, and conflicting symlinks are reported.

4. **`dotp`** reads `@require` and `@request` annotations across your dotfiles and installs packages using whichever manager is available on this machine.

`dotr` runs all four in a single pass. Run `dotr apply` whenever your dotfiles change.

---

## Quick start

```sh
# Apply everything — symlinks, packages, init.sh
dotr apply -f ~/dotfiles

# Wire init.sh into your shell
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc

# See what would change without touching anything
dotr apply -f ~/dotfiles --dry-run

# Check current state across all stages
dotr check -f ~/dotfiles
```

---

## Dotfiles repo layout

```
dotfiles/
  scripts/        ← shell scripts sourced into init.sh, in DAG order
  conf/           ← config files symlinked into $HOME
  bin/            ← executables symlinked onto $PATH
  env.yaml        ← your environment context (os, shell, context, etc.)
  packages.yaml   ← package registry
  .dot-dagger.yaml      ← metadata for files that can't carry annotations
```

### Naming conventions

**`dot-` prefix** is stripped from directory names in symlink destinations:

```
conf/dot-config/nvim/init.lua  →  ~/.config/nvim/init.lua
conf/dot-zshrc                 →  ~/.zshrc
```

**`nosync-` prefix** is stripped from logical names (useful for machine-specific dirs you don't commit):

```
nosync-work/scripts/aliases.sh  →  logical name: work.scripts.aliases
```

---

## Tools

### dotr — orchestrator

```sh
dotr apply -f ~/dotfiles             # full reconciliation
dotr apply -f ~/dotfiles --dry-run   # preview
dotr check -f ~/dotfiles             # validate all stages
```

### dote — environment

Owns `env.yaml`. Resolves the `Env` map used by all predicate evaluation.

```sh
dote show                    # print the fully resolved environment
dote show --env context=work # override a key
```

`dote show` is the first thing to reach for when a `@when` predicate isn't behaving as expected.

### dotd — init.sh generation

Owns `scripts/`. Evaluates predicates, resolves DAG, writes `init.sh`.

```sh
dotd apply -f ~/dotfiles                        # generate init.sh
dotd apply -f ~/dotfiles --dry-run              # preview
dotd check -f ~/dotfiles                        # validate DAG
dotd apply -f ~/dotfiles --init-file ~/init.sh  # custom output path
```

### dotl — symlink management

Owns `conf/` and `bin/`. Plans and applies symlinks. Reports drift.

```sh
dotl apply -f ~/dotfiles   # apply symlinks
dotl check -f ~/dotfiles   # report state (ok / missing / wrong-target / conflict)
dotl remove -f ~/dotfiles  # remove owned symlinks
```

Override the symlink root for a subtree via `.dot-dagger.yaml`:

```yaml
# conf/dot-config/nvim/.dot-dagger.yaml
directory:
  link_root: ~/.config/nvim
```

### dotp — packages

Owns `packages.yaml`. Reads `@require`/`@request` annotations and installs packages.

```sh
dotp install -f ~/dotfiles            # install declared packages
dotp check -f ~/dotfiles             # report status without installing
dotp list -f ~/dotfiles              # list all declared packages
dotp install -f ~/dotfiles --dry-run # preview
```

---

## Annotations

Annotations are comments at the top of a file. Scanning begins at the first line (skipping a shebang if present) and stops at the first blank line or non-comment line.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep

# --- script body below ---
export EDITOR=nvim
```

Shell files use `#`. C-style files use `//`. Any file format that supports comments works.

### Annotation reference

| Annotation | Owned by | Purpose |
|-----------|---------|---------|
| [`@when`](#when--inclusion-predicate) | all tools | Gate file inclusion on a predicate |
| [`@after`](#after--dag-ordering) | `dotd` | Declare a sourcing-order dependency |
| [`@name`](#name--logical-name) | `dotd` | Override the file's logical name |
| [`@symlink`](#symlink--explicit-destination) | `dotl` | Symlink to an explicit path |
| [`@retain-prefix`](#retain-prefix) | `dotl` | Opt out of `dot-` stripping |
| [`@require`](#require-and-request--packages) | `dotp` | Hard package gate |
| [`@request`](#require-and-request--packages) | `dotp` | Soft package ask |

---

### `@when` — inclusion predicate

A file with no `@when` is always active. A file with `@when` is active only if the predicate evaluates to true.

Multiple `@when` lines are **ANDed** together:

```sh
# @when os=macos
# @when context=work
# effective: os=macos AND context=work
```

#### Predicate DSL

```
predicate  = or_expr
or_expr    = and_expr (OR and_expr)*
and_expr   = atom (AND atom)*
atom       = "(" predicate ")"
           | call
           | comparison
call       = IDENT "(" IDENT ")"
comparison = KEY "=" VALUE ("," VALUE)*
```

`AND` binds tighter than `OR`. Use parentheses to override.

**Comparisons:**

```sh
# @when os=macos              # exact match
# @when os=macos,linux        # os is macos OR linux  (comma = same-key OR shorthand)
# @when shell=zsh,bash        # shell is zsh OR bash
```

**Operators:**

```sh
# @when os=macos OR os=linux           # either
# @when os=macos AND shell=zsh         # both
# @when os=macos AND (shell=zsh OR shell=bash)   # grouping
```

**Function calls:**

```sh
# @when exists(brew)                   # brew is on PATH
# @when installed(ripgrep)             # ripgrep binary is on PATH
# @when installable(ripgrep)           # ripgrep is in packages.yaml with an available manager
# @when os=macos AND exists(brew)      # combining function call with comparison
```

#### Built-in environment keys

| Key | Auto-detected | Values |
|-----|--------------|--------|
| `os` | Yes — from `runtime.GOOS` | `macos`, `linux` |
| `distro` | Yes — from `/etc/os-release` or `sw_vers` | `ubuntu`, `fedora`, `sequoia`, ... |
| `shell` | Yes — from `$SHELL` | `zsh`, `bash`, `fish`, ... |
| `context` | No — set in `env.yaml` | anything you define (`work`, `personal`, ...) |

Custom keys can be declared in `env.yaml`. Run `dote show` to see the full resolved map.

#### Built-in predicate functions

| Function | True when |
|----------|-----------|
| `exists(binary)` | `binary` is found on `$PATH` |
| `installed(pkg)` | the binary for `pkg` is found on `$PATH` (uses `packages.yaml` for binary name resolution) |
| `installable(pkg)` | `pkg` has an entry in `packages.yaml` with at least one manager available on `$PATH` |

---

### `@after` — DAG ordering

Controls the order scripts appear in `init.sh`. Only meaningful in `scripts/`.

```sh
# @after scripts/base/            # all active files under scripts/base/
# @after scripts/env/             # all active files under scripts/env/
# @after tmux.scripts.helpers     # one specific file, by logical name
```

- Path references ending in `/` expand to all active files under that path
- If no matching files are active, the dependency is silently ignored — never an error
- Files with no `@after` are ordered alphabetically by logical name within their topological frontier

---

### `@name` — logical name

Every file has a **logical name** derived from its path: strip `nosync-` and `dot-` prefixes from each component, strip the file extension from the last component.

```
scripts/helpers.sh          →  scripts.helpers
nosync-work/scripts/work.sh →  work.scripts.work
conf/dot-config/nvim/init.lua →  conf.config.nvim.init
```

`@name` replaces the entire derived name. Its primary use is **variant files** — two files that represent the same logical unit under mutually exclusive conditions:

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

Two active files with the same logical name is a conflict error. Predicates on variant files must be mutually exclusive.

---

### `@symlink` — explicit destination

Symlinks a file to an explicit path. Usually unnecessary — files in `conf/` and `bin/` are symlinked by convention. Use `@symlink` to override the conventional destination or to symlink a file outside those directories.

```sh
# @symlink ~/.gitconfig
```

Absolute paths are used as-is. Relative paths resolve against the effective `link_root` for that directory.

---

### `@retain-prefix`

By default, `dot-` is stripped from directory names when computing symlink destinations. `@retain-prefix` opts out of this for the **last path component** of the destination.

```sh
# conf/dot-tmux.conf  →  normally symlinked to ~/.tmux.conf
# With @retain-prefix:
# conf/dot-tmux.conf  →  symlinked to ~/.dot-tmux.conf
```

---

### `@require` and `@request` — packages

Registered by `dotp`. Available in any file across your dotfiles repo.

#### `@require pkg` — hard gate

The file is only active if `pkg` is installed or installable. If it can be installed, `dotp` installs it. If it can't be installed and isn't already present, `dotp` errors loudly.

```sh
# @require ripgrep
# This file is excluded unless ripgrep can be made available
```

#### `@request pkg` — soft ask

The file is always active. `dotp` installs `pkg` if it can; silently skips it if not.

```sh
# @request fzf
# This file is always active; fzf installed if possible
```

#### Packages in `@when`

`installed()` and `installable()` are also usable as predicate functions without triggering installation:

```sh
# @when installed(nvim)
# Active only if nvim is already installed — dotp won't install it
```

---

## Configuration files

### env.yaml

Declares your environment context. Lives at `~/dotfiles/env.yaml` or `~/.config/dot-dagger/env.yaml`.

```yaml
env:
  context: personal   # not auto-detected — must be set explicitly
```

Run `dote show` at any time to see the full resolved environment.

### packages.yaml

Registry of packages, package managers, and install commands. Lives at your dotfiles repo root.

```yaml
package_managers:
  brew:
    install:   brew install {package}
    uninstall: brew uninstall {package}
  apt:
    install:   apt install -y {package}
    uninstall: apt remove -y {package}
  dnf:
    install:   dnf install -y {package}
    uninstall: dnf remove -y {package}

packages:
  # Simple — same name across all managers
  fzf:
    brew: {}
    apt: {}

  # Binary name differs from package name
  ripgrep:
    binary: rg
    brew: {}
    apt: {}
    dnf: {}

  # Custom install command for one manager
  some-tool:
    brew:
      install: brew tap someorg/sometool && brew install some-tool
    apt: {}

  # No binary — custom existence check
  python-lib:
    check: "python3 -c 'import somelib'"
    pip:
      package: somelib
```

Package manager priority (which manager wins when multiple are available) is set in `packages.yaml` under `package_managers.priority`. Per-package `prefer` overrides the global order for a specific package:

```yaml
package_managers:
  priority: [brew, apt, dnf]   # global preference order
  brew:
    install: brew install {package}
    ...

packages:
  ripgrep:
    prefer: [dnf, brew]        # use dnf for this package if available
    binary: rg
    brew: {}
    dnf: {}
```

### .dot-dagger.yaml

Per-directory config for files that can't carry annotations — JSON, XML, binary, and anything else without a supported comment syntax. Can appear in any directory; `defaults` cascades to subdirectories.

```yaml
# Gate traversal of this entire directory
directory:
  when: "os=macos"
  link_root: ~/.config/nvim   # symlink root override for this subtree

# Applied to every file in this directory (ANDed with file's own @when)
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

  - path: settings.json
    when: "os=macos"
    symlink: settings.json    # relative to link_root
    retain_prefix: true
```

---

## License

MIT
