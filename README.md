# dotd

A dotfiles manager for people who use more than one machine.

### 📖 [Read the documentation → rocne.github.io/dot-dagger](https://rocne.github.io/dot-dagger/)

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
config/zsh/
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

**Convention over config.** Put files in `shellrc/`, `config/`, or `bin/` and they just work. Annotations and `.dagger` are for exceptions, not the common case.

**Composable subsystems.** Every subsystem works standalone. `dotd apply` composes them, but you can run individual stages, use only the pieces you need, and understand the system by reading one part at a time.

**Apply-time evaluation, not runtime conditionals.** Each file declares a condition — a test that evaluates to true or false for this machine. dotd checks all conditions once when you run `apply`. Your shell sources a pre-built `init.sh` with no branches — fast startup, predictable behavior.

---

## How it works

dotd runs four stages in sequence:

1. **Env** — detects your environment (OS, distro, shell) and loads any overrides from `env.yaml`. This produces the resolved environment used for all condition evaluation.
2. **Fileset** — walks the entire dotfiles repo, evaluates `@when` conditions, and builds the active file set for this machine. Files under `shellrc/`, `config/`, and `bin/` get special treatment (sourced, symlinked, added to PATH respectively); files anywhere else in the repo are included if they carry `@symlink` or `@source` annotations.
3. **Packages** — reads `@require` and `@request` annotations and installs packages using whichever manager is available on this machine.
4. **Symlinks + init.sh** — creates symlinks for `config/` and `bin/` files; resolves `@after` dependencies and writes a single `init.sh` that sources only the active scripts in the right order.

Most stages are also inspectable standalone: `dotd env show`, `dotd dag check`, `dotd package check`, `dotd compose check`.

---

## Install

### Homebrew (macOS / Linux)

```sh
brew install --cask rocne/tap/dot-dagger
```

Installs the `dotd` binary and tracks updates with `brew upgrade`. Each release
auto-bumps the cask in [rocne/homebrew-tap](https://github.com/rocne/homebrew-tap).

### apt (Debian / Ubuntu)

dotd is published to a hosted apt repo, so you get `apt install` by name plus
updates through `apt upgrade`. Add the repo once, then install:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.deb.sh' | sudo -E bash
sudo apt install dot-dagger
```

The setup script registers the repo and imports its signing key into a dedicated
keyring (`signed-by=`), so apt cryptographically verifies the repo index — no
`[trusted=yes]` shortcut. The packages themselves are independently GPG-signed.
The repo-signing public key is also committed here as
[`dotd-repo-signing-key.asc`](./dotd-repo-signing-key.asc) if you'd rather wire
the source up by hand.

### dnf (Fedora / RHEL)

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.rpm.sh' | sudo -E bash
sudo dnf install dot-dagger
```

Same model: the script configures the repo with `gpgcheck`/`repo_gpgcheck`
against the imported signing key.

### install.sh

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

Detects your OS and architecture, downloads the latest release, and installs to `~/.local/bin`. Requires only `curl`. Use this when you don't have Homebrew.

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --version v0.6.1
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

### Shell completion

`dotd completion` prints a completion script for the chosen shell. Pick the line
matching your shell:

```sh
# bash
dotd completion bash >> ~/.bashrc

# zsh
dotd completion zsh > "${fpath[1]}/_dotd"

# fish
dotd completion fish > ~/.config/fish/completions/dotd.fish

# powershell
dotd completion powershell | Out-String | Invoke-Expression
```

Reload your shell and tab-completion for commands, flags, and subcommands works.

---

## Verifying releases

Release artifacts are signed with [cosign](https://docs.sigstore.dev/) (keyless,
no private keys) and carry [SLSA build provenance](https://slsa.dev/). `install.sh`
verifies the signature automatically when `cosign` is installed; you can also
verify manually.

### Signature (requires cosign v2+)

```sh
TAG=v0.6.1   # the release you downloaded
BASE="https://github.com/rocne/dot-dagger/releases/download/$TAG"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt.sig"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt.pem"

cosign verify-blob \
  --certificate "dotd_${TAG}_checksums.txt.pem" \
  --signature   "dotd_${TAG}_checksums.txt.sig" \
  --certificate-identity-regexp "^https://github\.com/rocne/dot-dagger/\.github/workflows/_release\.yml@" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "dotd_${TAG}_checksums.txt"

# then confirm your archive matches the verified checksums file
# (Linux: sha256sum; macOS: shasum -a 256):
sha256sum --ignore-missing -c "dotd_${TAG}_checksums.txt" \
  || shasum -a 256 -c "dotd_${TAG}_checksums.txt" 2>/dev/null
```

### Build provenance

```sh
gh attestation verify dotd_${TAG}_linux_amd64.tar.gz --repo rocne/dot-dagger
```

(Requires the `gh` CLI; this repo's attestations are public.)

---

## Quick start

```sh
# First time on a machine: write config, then scaffold + wire your shell
dotd setup
dotd init

# Apply everything — symlinks, packages, init.sh
dotd apply

# See what would change without touching anything
dotd apply --dry-run

# Check current state across all stages
dotd check
```

`dotd setup` writes `config.yaml` (and `env.yaml` if absent). `dotd init`
scaffolds the convention directories in your dotfiles repo and offers to
append the `init.sh` source line to your shell RC file.

---

## Dotfiles repo layout

```
dotfiles/
  shellrc/          ← shell rc fragments sourced into init.sh, in dependency order
  config/           ← config files symlinked into ~/.config
  bin/              ← executables symlinked onto $PATH
  env.yaml          ← your environment context (os, shell, context, etc.)
  packages.yaml     ← package registry
  .dagger           ← per-directory config for files that can't carry annotations
```

dotd walks the entire repo. Files under `shellrc/`, `config/`, or `bin/` are picked up automatically based on their location. Files elsewhere are included if they carry `@symlink` or `@source` annotations. Annotations (comments at the top of a file) control conditions, ordering, and package requirements.

### Naming conventions

**`dot-` prefix** turns into a leading `.` when computing symlink destinations, so `dot-` prefixed names become hidden paths:

```
config/nvim/init.lua     →  ~/.config/nvim/init.lua
config/dot-tmux.conf     →  ~/.config/.tmux.conf
```

Note that `config/` links into `~/.config` by default, so a `dot-` prefixed file there becomes a hidden file *inside* `~/.config`. For a dotfile that belongs at the top of your home directory, use an explicit destination instead: `# @symlink ~/.tmux.conf`.

**`nosync-` prefix** is stripped from path components when computing a file's identity. This is useful for machine-specific directories you don't want to commit — they still participate in the process but don't pollute logical names:

```
nosync-work/shellrc/aliases.sh  →  identity: work.shellrc.aliases
```

---

## Commands

### dotd apply

Full reconciliation: resolves environment, installs packages, applies symlinks, writes init.sh.

```sh
dotd apply -f ~/dotfiles
dotd apply -f ~/dotfiles --dry-run
dotd apply -f ~/dotfiles --debug
dotd apply -f ~/dotfiles --env context=work
```

### dotd check

Validates all stages without making any changes.

```sh
dotd check -f ~/dotfiles
dotd check -f ~/dotfiles --debug
```

### dotd setup — onboarding

Interactive wizard that writes `config.yaml` (and `env.yaml` if absent). It does not touch your dotfiles repo or shell RC — run `dotd init` next for that.

```sh
dotd setup                            # interactive — walks through each step
dotd setup --non-interactive          # accept all defaults (alias: -n)
dotd setup -n --files ~/my-dotfiles   # scripted, with a custom dotfiles directory
```

### dotd adopt — bring a file in

Moves an existing file into your dotfiles repo — inferring the right destination directory — and replaces the original with a symlink.

```sh
dotd adopt ~/.bashrc              # infers config/dot-bashrc
dotd adopt ~/bin/my-script        # infers bin/my-script (marked executable)
dotd adopt ~/.gitconfig --to config/dot-gitconfig-personal
dotd adopt ~/.zshrc --yes         # accept inferred destination without prompting
```

**Inference rules** (checked in order):

| File characteristic | Destination |
|--------------------|------------|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` extension | `shellrc/<name>` |
| Hidden file (`.bashrc`, `.zshrc`, …) | `config/dot-<name>` |
| `.conf`, `.config`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` extension | `config/<name>` |
| Anything else | Error — use `--to` |

### dotd list — inspect the file set

```sh
dotd list                          # list active nodes for this machine
dotd list --inactive               # show nodes filtered out by predicates
dotd list --env os=macos           # preview for a different environment
dotd list --json                   # machine-readable output
```

`dotd list` is useful for understanding which files are active and why. `--inactive` shows the nodes that predicates filtered out.

### dotd env — environment

```sh
dotd env show                  # print all resolved key=value pairs
dotd env get os                # print a single key's value
dotd env set context work      # write a key to env.yaml
dotd env diff                  # keys where env.yaml overrides detected values
```

`dotd env show` is the first thing to reach for when a `@when` condition isn't behaving as expected.

### dotd dag — load order

```sh
dotd dag check                              # print active nodes in dependency order
dotd dag check --env os=macos               # preview for a different environment
dotd dag check --json | jq -r '.[].logical_name'
```

init.sh itself is written by `dotd apply`. Symlinks are managed by `dotd apply` / `dotd check` / `dotd unapply` — there is no separate link command.

### dotd unapply — undo

```sh
dotd unapply                   # preview, then prompt for confirmation
dotd unapply --dry-run         # preview only
dotd unapply --yes             # skip confirmation prompt
```

### dotd package — packages

```sh
dotd package generate          # generate shell install script (preview)
dotd package generate | sudo sh                 # install packages
dotd package generate > packages-install.sh    # write to file
dotd package check             # report status without installing
dotd package list              # list all declared packages
```

---

## Annotations

Annotations are metadata written as comments at the top of a file. They declare conditions, dependencies, package requirements, and other per-file behavior. dotd reads them at apply time — they have no effect at shell startup.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after shellrc/base/
# @require ripgrep

export EDITOR=nvim
```

Scanning begins at the first line. If the first line is a shebang (`#!/bin/bash`), it is skipped. Scanning then reads contiguous comment lines and stops at the first blank line or non-comment line. Shell files use `#`. C-style files use `//`.

Every file has a **logical name** derived from its path: `nosync-` and `dot-` prefixes are stripped from each path component, the file extension is stripped from the filename, and the remaining components are joined with `.`:

```
shellrc/helpers.sh              →  shellrc.helpers
nosync-work/shellrc/work.sh     →  work.shellrc.work
config/dot-config/nvim/init.lua →  config.config.nvim.init
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
# shellrc/aliases-macos.sh
# @name shellrc.aliases
# @when os=macos
```

```sh
# shellrc/aliases-linux.sh
# @name shellrc.aliases
# @when os=linux
```

Only one can be active at a time. Two active files with the same logical name is an error.

---

### `@after` — load order

Controls the order scripts appear in `init.sh`. Only meaningful in `shellrc/`.

```sh
# @after shellrc/base/            # all active files under shellrc/base/
# @after shellrc/env/
# @after shellrc.helpers          # one specific file, by logical name
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
# @source     # force-source even if not in shellrc/
```

| Annotation | In load order? | Symlinked? | Sourced in init.sh? |
|-----------|---------------|-----------|-------------------|
| _(none)_ in `shellrc/` | Yes | No | Yes |
| _(none)_ in `config/` | No | Yes | No |
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

### .dagger

Per-directory config. Controls how files in a directory are processed, and declares metadata for files that can't carry annotations (JSON, XML, binaries, etc.). Settings cascade downward; `defaults` apply to the whole subtree. (`.dotd.yaml` is the legacy name — rename to `.dagger`.)

```yaml
when: os=macos
link_root: $config/nvim

defaults:
  when: context=work
  actions:
    - link

files:
  settings.json:
    when: os=macos
    actions:
      - link(~/.config/nvim/settings.json)
```

See the [`.dagger` reference](https://rocne.github.io/dot-dagger/reference/dagger/) for all fields, including `compose:` targets and convention-directory overrides.

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
