# dotd

A dotfiles manager for people who use more than one machine.

If your setup is a single laptop running one OS and one shell, a handful of symlinks and a `.zshrc` is probably fine. But if you work across a personal Mac, a work Linux box, maybe a remote server — each with different shells, different package managers, different software installed — keeping one set of dotfiles that behaves correctly everywhere gets complicated fast.

The kind of problems dotd is designed for:

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

dotd's approach: **annotate files, not shell code**. Each file declares when it should be active and what it depends on. dotd evaluates those declarations once when you run `apply`, builds the active file set for this machine, and writes a clean `init.sh` with no runtime branches.

```sh
#!/bin/bash
# @when os=macos
# @after scripts/base/
# @require ripgrep

alias ls='ls -G'
export HOMEBREW_PREFIX="/opt/homebrew"
```

The same annotation system drives symlink management and package installation. One mental model, one place to declare intent.

---

## How dotd compares

| | [GNU Stow](https://www.gnu.org/software/stow/) + scripts | [dotbot](https://github.com/anishathalye/dotbot) | [chezmoi](https://www.chezmoi.io/) | dotd |
|---|---|---|---|---|
| Symlinks | ✓ | ✓ | ✗ (copies) | ✓ |
| Per-file conditions | ✗ | ✗ | Templates | Annotations |
| Dependency-ordered init.sh | ✗ | ✗ | ✗ | ✓ |
| Package management | ✗ | Plugins | ✗ | ✓ (`@require`/`@request`) |
| Work/personal separation | Manual | Manual | Encryption | `@when context=work` |
| Multi-shell | Manual | Manual | Templates | `@when shell=zsh` |
| Central manifest | ✗ | ✓ | ✓ | ✗ (annotations in files) |
| Shell startup cost | Varies | Varies | Low | Low (conditions evaluated at apply time, not runtime) |

**Stow + scripts** is the baseline — symlinks work great, but conditions and ordering are all manual shell scripting. dotd is what you reach for when the scripts start to sprawl.

**dotbot** adds a structured YAML manifest for symlinks and actions. It's good at idempotent setup but has no concept of per-file conditions — you'd still write shell scripts or plugins for conditional behavior.

**chezmoi** is the most fully-featured alternative. It uses templates and copies files rather than symlinking, supports encryption for secrets, and has a large feature surface. If you need encrypted secrets in your dotfiles repo, chezmoi is probably the right choice. dotd's trade-off is simplicity: symlinks over copies, annotations over templates, no encryption.

The core dotd bet: **conditions belong on files, not in shell code or central manifests**. A file knows whether it applies to macOS. It knows it needs `ripgrep` installed. It knows it should be sourced after the base environment is set up. Keeping that knowledge with the file means you can look at any file and immediately understand when and how it's used — without cross-referencing a central config.

---

## Philosophy

**One annotation, one concern.** Each annotation does exactly one thing. `@when` controls inclusion. `@after` controls ordering. `@require` gates on a package. They compose but don't interfere.

**Convention over config.** Put files in `scripts/`, `conf/`, or `bin/` and they just work. Annotations and `.dotd.yaml` are for exceptions, not the common case.

**Composable subsystems.** Every subsystem works standalone. `dotd apply` composes them, but you can run individual stages, use only the pieces you need, and understand the system by reading one part at a time.

**Apply-time evaluation, not runtime conditionals.** Each file declares a condition — a test that evaluates to true or false for this machine. dotd checks all conditions once when you run `apply`. Your shell sources a pre-built `init.sh` with no branches — fast startup, predictable behavior.

---

## How it works

dotd runs four stages in sequence:

1. **Env** — detects your environment (OS, distro, shell) and loads any overrides from `env.yaml`. This produces the resolved environment used for all condition evaluation.
2. **Fileset** — walks `scripts/`, `conf/`, and `bin/`, evaluates `@when` conditions, and builds the active file set for this machine.
3. **Packages** — reads `@require` and `@request` annotations and installs packages using whichever manager is available on this machine.
4. **Symlinks + init.sh** — creates symlinks for `conf/` and `bin/` files; resolves `@after` dependencies and writes a single `init.sh` that sources only the active scripts in the right order.

Each stage is also available standalone: `dotd env`, `dotd dag`, `dotd link`, `dotd package`.

---

## Install

### install.sh (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs to `~/.local/bin`. Requires [gh CLI](https://cli.github.com).

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --version v0.2.0
```

Install to a custom directory:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --dir /usr/local/bin
```

### From source

```sh
git clone https://github.com/rocne/dot-dagger
cd dot-dagger
go install ./cmd/dotd
```

---

## Quick start

```sh
# Apply everything — symlinks, packages, init.sh
dotd apply -f ~/dotfiles

# Wire init.sh into your shell (pick your shell)
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc   # zsh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.bashrc  # bash

# See what would change without touching anything
dotd apply -f ~/dotfiles --dry-run

# Check current state across all stages
dotd check -f ~/dotfiles
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
  .dotd.yaml  ← per-directory config for files that can't carry annotations
```

Any file in `scripts/`, `conf/`, or `bin/` is picked up automatically. Annotations (comments at the top of a file) control conditions, ordering, and package requirements.

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

---

## Commands

### dotd apply

Full reconciliation: resolves environment, installs packages, applies symlinks, writes init.sh.

```sh
dotd apply -f ~/dotfiles
dotd apply -f ~/dotfiles --dry-run
dotd apply -f ~/dotfiles --verbose
dotd apply -f ~/dotfiles --env context=work
```

### dotd check

Validates all stages without making any changes.

```sh
dotd check -f ~/dotfiles
dotd check -f ~/dotfiles --verbose
```

### dotd setup — onboarding

Interactive setup that scaffolds a dotfiles repo, writes config files, and wires up your shell.

```sh
dotd setup                  # interactive — walks through each step
dotd setup --yes            # non-interactive, accept all defaults
dotd setup -f ~/my-dotfiles # specify a dotfiles directory
```

### dotd adopt — bring a file in

Copies an existing file into your dotfiles repo, inferring the right destination directory.

```sh
dotd adopt ~/.bashrc              # infers conf/dot-bashrc
dotd adopt ~/bin/my-script        # infers bin/my-script (marked executable)
dotd adopt ~/.gitconfig --to conf/dot-gitconfig-personal
dotd adopt ~/.zshrc --yes         # accept inferred destination without prompting
```

**Inference rules** (checked in order):

| File characteristic | Destination |
|--------------------|------------|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` extension | `scripts/<name>` |
| Hidden file (`.bashrc`, `.zshrc`, …) | `conf/dot-<name>` |
| `.conf`, `.config`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` extension | `conf/<name>` |
| Anything else | Error — use `--to` |

### dotd env — environment

```sh
dotd env show                  # print all resolved key=value pairs
dotd env get os                # print a single key's value
dotd env set context=work      # write a key to env.yaml
```

`dotd env show` is the first thing to reach for when a `@when` condition isn't behaving as expected.

### dotd dag — init.sh generation

```sh
dotd dag apply                         # write init.sh
dotd dag apply --dry-run               # preview without writing
dotd dag apply --init-file ~/init.sh   # custom output path
dotd dag check                         # validate without writing
dotd dag check --verbose               # show numbered load order
```

### dotd link — symlinks

```sh
dotd link apply                # create/update symlinks
dotd link apply --dry-run      # preview
dotd link apply --force        # overwrite conflicting files
dotd link check                # report state
dotd link check --verbose      # include ok symlinks in output
dotd link remove               # remove owned symlinks
```

### dotd package — packages

```sh
dotd package generate          # generate shell install script (preview)
dotd package generate | sudo sh  # install packages
dotd package generate -o packages-install.sh  # write to file
dotd package check             # report status without installing
dotd package list              # list all declared packages
```

---

## Annotations

Annotations are metadata written as comments at the top of a file. They declare conditions, dependencies, package requirements, and other per-file behavior. dotd reads them at apply time — they have no effect at shell startup.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep

export EDITOR=nvim
```

Scanning begins at the first line. If the first line is a shebang (`#!/bin/bash`), it is skipped. Scanning then reads contiguous comment lines and stops at the first blank line or non-comment line. Shell files use `#`. C-style files use `//`.

Every file has a **logical name** derived from its path: `nosync-` and `dot-` prefixes are stripped from each path component, the file extension is stripped from the filename, and the remaining components are joined with `.`:

```
scripts/helpers.sh              →  scripts.helpers
nosync-work/scripts/work.sh     →  work.scripts.work
conf/dot-config/nvim/init.lua   →  conf.config.nvim.init
```

### Annotation reference

| Annotation | Purpose |
|-----------|---------|
| [`@when`](#when--condition) | Gate file inclusion on a condition |
| [`@name`](#name--override-logical-name) | Override the file's logical name |
| [`@after`](#after--load-order) | Declare a load-order dependency |
| [`@symlink`](#symlink--explicit-destination) | Symlink to an explicit path |
| [`@retain-prefix`](#retain-prefix) | Keep `dot-`/`nosync-` prefix on the filename |
| [`@require`](#require-and-request--packages) | Hard package gate |
| [`@request`](#require-and-request--packages) | Soft package ask |
| [`@disable`](#disable-no-source-and-source--sourcing-control) | Exclude from all processing |
| [`@no-source`](#disable-no-source-and-source--sourcing-control) | In load order, not sourced |
| [`@source`](#disable-no-source-and-source--sourcing-control) | Force-source regardless of directory |

---

### `@when` — condition

A file with no `@when` is always active. A file with `@when` is only included if the condition is true for this machine.

```sh
# @when os=macos
# @when os=macos AND context=work
# @when os=macos,linux               # comma = OR shorthand for the same key
# @when exists(brew)
# @when os=macos AND (shell=zsh OR shell=bash)
```

Multiple `@when` lines are ANDed together.

#### Built-in environment keys

| Key | Auto-detected | Values |
|-----|--------------|--------|
| `os` | Yes | `macos`, `linux` |
| `distro` | Yes | `ubuntu`, `fedora`, `macos`, ... |
| `shell` | Yes | `zsh`, `bash`, `fish`, ... |
| `context` | No — set in `env.yaml` | anything you define |

#### Built-in functions

| Function | True when |
|----------|-----------|
| `exists(binary)` | `binary` is found on `$PATH` |
| `installed(pkg)` | the binary for `pkg` is on `$PATH` |
| `installable(pkg)` | `pkg` has an entry in `packages.yaml` with an available manager |

---

### `@name` — override logical name

Primarily used for **variant files** — two files that represent the same thing under mutually exclusive conditions:

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

Only one can be active at a time. Two active files with the same logical name is an error.

---

### `@after` — load order

Controls the order scripts appear in `init.sh`. Only meaningful in `scripts/`.

```sh
# @after scripts/base/            # all active files under scripts/base/
# @after scripts/env/
# @after scripts.helpers          # one specific file, by logical name
```

A path ending in `/` expands to all active files under that path. If no matching files are active, the dependency is silently ignored.

---

### `@symlink` — explicit destination

Symlinks a file to an explicit path instead of the conventional destination.

```sh
# @symlink ~/.gitconfig
```

---

### `@retain-prefix`

Keeps the `dot-` or `nosync-` prefix on the filename in the symlink destination:

```sh
# conf/dot-tmux.conf  →  normally: ~/.tmux.conf
# with @retain-prefix: ~/.dot-tmux.conf
```

---

### `@require` and `@request` — packages

```sh
# @require ripgrep   # hard gate: file excluded unless ripgrep can be made available
# @request fzf       # soft ask: file always active; fzf installed if possible
```

---

### `@disable`, `@no-source`, and `@source` — sourcing control

```sh
# @disable    # exclude from all processing
# @no-source  # in load order but not sourced in init.sh
# @source     # force-source even if not in scripts/
```

| Annotation | In load order? | Symlinked? | Sourced in init.sh? |
|-----------|---------------|-----------|-------------------|
| _(none)_ in `scripts/` | Yes | No | Yes |
| _(none)_ in `conf/` | No | Yes | No |
| `@no-source` | Yes | As normal | **No** |
| `@source` | Yes | As normal | **Yes** |
| `@disable` | **No** | **No** | **No** |

---

## Configuration files

### env.yaml

Declares your environment context. Lives at `~/dotfiles/env.yaml` or `~/.config/dot-dagger/env.yaml`.

```yaml
env:
  context: personal
```

Run `dotd env show` to see the full resolved environment.

### packages.yaml

Registry of packages and package managers.

```yaml
package_managers:
  brew:
    install:   brew install {package}
    uninstall: brew uninstall {package}
  apt:
    install:   apt install -y {package}
    uninstall: apt remove -y {package}

packages:
  ripgrep:
    binary: rg
    brew: {}
    apt: {}
```

### .dotd.yaml

Per-directory config for files that can't carry annotations (JSON, XML, binaries, etc.).

```yaml
dotd:
  when: "os=macos"
  defaults:
    when: "context=work"
  files:
    - path: dot-gitconfig-work
      when: "context=work"
      symlink: ~/.gitconfig

link:
  link_root: ~/.config/nvim
```

---

## Development

**Run tests:**

```sh
go test ./...
go test -tags integration ./cmd/dotd/
```

**Serve docs locally:**

```sh
pip install -r docs/requirements.txt
mkdocs serve
```

---

## License

MIT
