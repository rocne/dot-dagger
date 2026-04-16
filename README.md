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

dotr's approach: **annotate files, not shell code**. Each file declares when it should be active and what it depends on. dotr evaluates those declarations once when you run `apply`, builds the active file set for this machine, and writes a clean `init.sh` with no runtime branches.

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
| Dependency-ordered init.sh | ✗ | ✗ | ✗ | ✓ |
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

**Apply-time evaluation, not runtime conditionals.** Each file declares a condition — a test that evaluates to true or false for this machine. dotr checks all conditions once when you run `apply`. Your shell sources a pre-built `init.sh` with no branches — fast startup, predictable behavior.

---

## How it works

dotr is made up of four tools that each own one stage of the process:

1. **`dote`** detects your environment — OS, distro, shell — and loads any overrides from `env.yaml`. This produces the resolved environment used for all condition evaluation.

2. **`dotd`** walks `scripts/`, evaluates `@when` conditions against the resolved environment, and resolves `@after` dependencies into a load order. It writes a single `init.sh` that sources only the active scripts in the right order.

3. **`dotl`** walks `conf/` and `bin/`, plans symlinks into `$HOME` and `$PATH`, and applies them. It detects drift — missing, wrong-target, and conflicting symlinks are reported.

4. **`dotp`** reads `@require` and `@request` annotations across your dotfiles and installs packages using whichever manager is available on this machine.

`dotr` runs all four in a single pass. Run `dotr apply` whenever your dotfiles change.

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

## Quick start

```sh
# Apply everything — symlinks, packages, init.sh
dotr apply -f ~/dotfiles

# Wire init.sh into your shell (pick your shell)
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc   # zsh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.bashrc  # bash

# See what would change without touching anything
dotr apply -f ~/dotfiles --dry-run

# Check current state across all stages
dotr check -f ~/dotfiles
```

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

Any file in `scripts/`, `conf/`, or `bin/` is picked up automatically. Annotations (comments at the top of a file — see [Annotations](#annotations) below) control conditions, ordering, and package requirements.

### Naming conventions

**`dot-` prefix** is stripped when computing symlink destinations, so files named with a `dot-` prefix become hidden files in `$HOME`:

```
conf/dot-config/nvim/init.lua  →  ~/.config/nvim/init.lua
conf/dot-zshrc                 →  ~/.zshrc
```

**`nosync-` prefix** is stripped from path components when computing a file's identity. This is useful for machine-specific directories you don't want to commit — they still participate in the process but don't pollute logical names:

```
nosync-work/scripts/aliases.sh  →  identity: work.scripts.aliases
```

Both prefixes are explained further in the [Annotations](#annotations) section.

---

## Tools

### dotr — orchestrator

Runs all four tools in sequence: environment → packages → symlinks → init.sh.

```sh
dotr apply -f ~/dotfiles             # full reconciliation
dotr apply -f ~/dotfiles --dry-run   # preview
dotr check -f ~/dotfiles             # validate all stages
```

### dotr setup — onboarding

Interactive setup that scaffolds a dotfiles repo, writes config files, and wires up your shell.

```sh
dotr setup                  # interactive — walks through each step
dotr setup --yes            # non-interactive, accept all defaults
dotr setup -f ~/my-dotfiles # specify a dotfiles directory
```

Steps through: dotfiles directory path, `env.yaml` location, `init.sh` output path, and which package managers to pre-populate in `packages.yaml`. After scaffolding, it detects your shell config file and offers to append the `source` line automatically.

---

### dotr adopt — bring a file in

Copies an existing file into your dotfiles repo, inferring the right destination directory from the file's name and properties.

```sh
dotr adopt ~/.bashrc              # infers conf/dot-bashrc
dotr adopt ~/bin/my-script        # infers bin/my-script (marked executable)
dotr adopt ~/setup.sh             # infers scripts/setup.sh (.sh extension)
dotr adopt ~/.config/foo/bar.toml # infers conf/bar.toml (.toml extension)

# Override the destination explicitly
dotr adopt ~/.gitconfig --to conf/dot-gitconfig-personal

# Accept the inferred destination without prompting
dotr adopt ~/.bashrc --yes
```

**Inference rules** (checked in order):

| File characteristic | Destination |
|--------------------|------------|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` extension | `scripts/<name>` |
| Hidden file (`.bashrc`, `.zshrc`, …) | `conf/dot-<name>` — the `dot-` prefix means it will symlink back as a hidden file |
| `.conf`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` extension | `conf/<name>` |
| Anything else | Error — use `--to` to specify the destination |

After copying, `adopt` offers to remove the original (in interactive mode). Run `dotr apply` afterwards to create the symlink back to `$HOME`.

### dotr subcommands — condition-filtered stages

`dotr` exposes each stage as a subcommand so you can run a single stage with conditions applied, without running the full pipeline:

| Subcommand | Stage | Equivalent standalone |
|---|---|---|
| `dotr env show/get/set` | Environment | `dote show/get/set` |
| `dotr dag apply/check` | init.sh generation | `dotd apply/check` |
| `dotr link apply/check/remove` | Symlinks | `dotl apply/check/remove` |
| `dotr package install/check/list` | Packages | `dotp install/check/list` |

The difference from the standalone tools: `dotr` subcommands evaluate `@when` conditions first, so only files active on this machine are processed. The standalone tools (`dotd`, `dotl`, `dotp`) operate unconditionally — useful for introspection or scripting.

```sh
dotr env show                       # show resolved environment
dotr env get os                     # get a single key
dotr env set context=work           # write to env.yaml
dotr dag apply --dry-run            # preview init.sh without writing
dotr dag check --verbose            # show load order
dotr link check                     # report symlink state
dotr package check                  # report package status
```

---

### dote — environment

Owns `env.yaml`. Resolves the environment map used by all condition evaluation.

```sh
dote show                    # print the fully resolved environment
dote show --env context=work # override a key
```

`dote show` is the first thing to reach for when a `@when` condition isn't behaving as expected.

### dotd — init.sh generation

Owns `scripts/`. Evaluates conditions, resolves load order, writes `init.sh`.

```sh
dotd apply -f ~/dotfiles                        # generate init.sh
dotd apply -f ~/dotfiles --dry-run              # preview
dotd check -f ~/dotfiles                        # validate
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
dotl:
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

Annotations are metadata written as comments at the top of a file. They declare conditions, dependencies, package requirements, and other per-file behavior. dotr reads them at apply time — they have no effect at shell startup.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep

# --- script body below ---
export EDITOR=nvim
```

Scanning begins at the first line. If the first line is a shebang (`#!/bin/bash` — the line that tells the OS which interpreter to use), it is skipped. Scanning then reads contiguous comment lines and stops at the first blank line or non-comment line. Shell files use `#`. C-style files use `//`. Any file format with comments works.

Every file has a **logical name** derived from its path: `nosync-` and `dot-` prefixes are stripped from each path component, the file extension is stripped from the filename, and the remaining components are joined with `.`:

```
scripts/helpers.sh              →  scripts.helpers
nosync-work/scripts/work.sh     →  work.scripts.work
conf/dot-config/nvim/init.lua   →  conf.config.nvim.init
```

The logical name is used for dependency declarations and to identify variant files. See [`@name`](#name--override-logical-name) for how to override it.

### Annotation reference

| Annotation | Owned by | Purpose |
|-----------|---------|---------|
| [`@when`](#when--condition) | all tools | Gate file inclusion on a condition |
| [`@name`](#name--override-logical-name) | `dotd` | Override the file's logical name |
| [`@after`](#after--load-order) | `dotd` | Declare a load-order dependency |
| [`@symlink`](#symlink--explicit-destination) | `dotl` | Symlink to an explicit path |
| [`@retain-prefix`](#retain-prefix) | `dotl` | Keep `dot-`/`nosync-` prefix on the filename instead of stripping it |
| [`@require`](#require-and-request--packages) | `dotp` | Hard package gate |
| [`@request`](#require-and-request--packages) | `dotp` | Soft package ask |
| [`@disable`](#disable-no-source-and-source--sourcing-control) | all tools | Exclude file from all processing |
| [`@no-source`](#disable-no-source-and-source--sourcing-control) | `dotd` | Keep in load order but omit from init.sh |
| [`@source`](#disable-no-source-and-source--sourcing-control) | `dotd` | Force-source regardless of directory |

---

### `@when` — condition

A file with no `@when` is always active. A file with `@when` is only included if the condition is true for this machine.

```sh
# @when os=macos                         # only on macOS
# @when os=macos AND context=work        # macOS and work context
# @when os=macos,linux                   # macOS or Linux (comma = OR shorthand)
# @when exists(brew)                     # only if brew is on PATH
# @when os=macos AND (shell=zsh OR shell=bash)
```

Multiple `@when` lines are ANDed together:

```sh
# @when os=macos
# @when context=work
# effective: os=macos AND context=work
```

#### Conditions reference

A condition is an expression that evaluates to true or false. The simplest form is a key-value comparison:

```sh
# @when os=macos              # exact match
# @when os=macos,linux        # os is macos OR linux
# @when shell=zsh,bash        # shell is zsh OR bash
```

Conditions can be combined with `AND` and `OR`. `AND` binds more tightly than `OR`; use parentheses to override:

```sh
# @when os=macos OR os=linux
# @when os=macos AND shell=zsh
# @when os=macos AND (shell=zsh OR shell=bash)
```

Conditions can also call built-in functions:

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

Custom keys can be declared in `env.yaml`. Run `dote show` to see the full resolved environment.

#### Built-in functions

| Function | True when |
|----------|-----------|
| `exists(binary)` | `binary` is found on `$PATH` |
| `installed(pkg)` | the binary for `pkg` is found on `$PATH` (uses `packages.yaml` for binary name resolution) |
| `installable(pkg)` | `pkg` has an entry in `packages.yaml` with at least one manager available on `$PATH` |

---

### `@name` — override logical name

Every file has a logical name derived from its path (described above). `@name` replaces the entire derived name.

Its main use is **variant files** — two files that represent the same thing under mutually exclusive conditions:

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

Since only one can be active at a time, they share the same logical name without conflict. Two active files with the same logical name is an error — conditions on variant files must be mutually exclusive.

---

### `@after` — load order

Controls the order scripts appear in `init.sh`. Only meaningful in `scripts/`.

```sh
# @after scripts/base/            # depends on all active files under scripts/base/
# @after scripts/env/             # depends on all active files under scripts/env/
# @after scripts.helpers          # depends on one specific file, by logical name
```

- A path ending in `/` expands to all active files under that path
- A reference without a trailing `/` is matched against logical names
- If no matching files are active, the dependency is silently ignored — it's never an error to declare a dependency on something that doesn't exist on this machine
- Files with no `@after` are ordered alphabetically by logical name within their position in the dependency graph

---

### `@symlink` — explicit destination

Symlinks a file to an explicit path instead of the conventional destination. Usually unnecessary — files in `conf/` and `bin/` are symlinked automatically by convention. Use `@symlink` to override the destination or to symlink a file that lives outside those directories.

```sh
# @symlink ~/.gitconfig
```

Absolute paths are used as-is. Relative paths resolve against the effective `link_root` for that directory.

---

### `@retain-prefix`

By default, both `dot-` and `nosync-` are stripped from every path component when computing logical names and symlink destinations. `@retain-prefix` opts out of this for the **filename** — both prefixes are kept as-is.

```sh
# conf/dot-tmux.conf       →  normally symlinked to ~/.tmux.conf
# conf/nosync-private.conf →  normally symlinked to ~/.private.conf

# With @retain-prefix:
# conf/dot-tmux.conf       →  symlinked to ~/.dot-tmux.conf
# conf/nosync-private.conf →  symlinked to ~/.nosync-private.conf
```

Directory components above the filename are always stripped regardless of `@retain-prefix`.

---

### `@require` and `@request` — packages

These annotations declare that a file depends on an external package. `dotp` reads them across your entire dotfiles repo and installs what's needed.

#### `@require pkg` — hard gate

The file is only active if `pkg` is installed or can be installed. If it can be installed, `dotp` installs it automatically. If it can't be installed and isn't already present, `dotp` errors loudly.

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

#### Using package state as a condition

`installed()` and `installable()` can also be used in `@when` without triggering installation:

```sh
# @when installed(nvim)
# Active only if nvim is already installed — dotp won't try to install it
```

---

### `@disable`, `@no-source`, and `@source` — sourcing control

By convention, files in `scripts/` are sourced in `init.sh` and files in `conf/` are symlinked. These three annotations let you override that behavior for specific files.

#### `@disable` — exclude from all processing

Removes the file from every stage: no symlinks, no load order, no sourcing, no package checks. Equivalent to the file not existing, as far as dotr is concerned.

```sh
# @disable
# This file is completely ignored by dotr regardless of which directory it lives in
```

Useful for keeping a file in your dotfiles repo (for reference, backup, or future use) without having it take effect anywhere.

#### `@no-source` — in load order, not sourced

Keeps the file in the dependency graph so other files can declare `@after` it, but excludes it from `init.sh`. Only meaningful for files that would otherwise be sourced (files in `scripts/`, or files with `@source`).

```sh
# scripts/helpers.sh
# @no-source
# Other scripts can @after scripts.helpers, but helpers.sh itself isn't sourced directly
```

#### `@source` — force-source from any directory

Forces a file into `init.sh` even if it isn't in `scripts/`. Works with any file anywhere in your dotfiles repo.

```sh
# conf/dot-config/shell/extras.sh
# @source
# This conf/ file will be sourced in init.sh despite not being in scripts/
```

Use `@source` when you have a shell script that also needs to be symlinked as a config file, or when your repo layout doesn't fit the `scripts/` convention for a particular file.

#### Summary

| Annotation | In load order? | Symlinked? | Sourced in init.sh? |
|-----------|---------------|-----------|-------------------|
| _(none)_ in `scripts/` | Yes | No | Yes |
| _(none)_ in `conf/` | No | Yes | No |
| `@no-source` | Yes | As normal | **No** |
| `@source` | Yes | As normal | **Yes** |
| `@disable` | **No** | **No** | **No** |

`.dot-dagger.yaml` equivalents for files that can't carry annotations: `disable: true`, `no_source: true`, `source: true` in a `files:` entry.

---

## Configuration files

### env.yaml

Declares your environment context. Lives at `~/dotfiles/env.yaml` or `~/.config/dot-dagger/env.yaml`.

```yaml
env:
  context: personal   # not auto-detected — must be set explicitly
```

Most environment keys (`os`, `shell`, `distro`) are detected automatically. `context` is the main thing to set here. Run `dote show` at any time to see the full resolved environment.

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

When multiple package managers are available, the one to use is determined by the `priority` list. A per-package `prefer` overrides the global order for that package:

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

Some files can't carry annotations — JSON, XML, compiled binaries, and anything else that doesn't have a comment syntax dotr recognizes. `.dot-dagger.yaml` is a per-directory config file that lets you provide the same metadata for those files.

It can appear in any directory. The `defaults` section cascades down to all files in that directory and its subdirectories.

```yaml
# dotd section: directory-level conditions and per-file overrides
dotd:
  # Skip this entire directory unless the condition is true
  when: "os=macos"

  # Apply this condition to every file in the directory (ANDed with each file's own @when)
  defaults:
    when: "context=work"

  # Per-file metadata — same fields as the equivalent annotations
  files:
    - path: dot-gitconfig-work
      when: "context=work"
      symlink: ~/.gitconfig

    - path: dot-gitconfig-personal
      when: "context=personal"
      symlink: ~/.gitconfig

    - path: settings.json
      when: "os=macos"
      retain_prefix: true

    # Sourcing control equivalents
    - path: some-helper.json
      disable: true        # equivalent to @disable
    - path: loader.sh
      no_source: true      # equivalent to @no-source
    - path: extras.sh
      source: true         # equivalent to @source

# dotl section: symlink root override for this directory and its children
dotl:
  link_root: ~/.config/nvim
```

---

## License

MIT
