# dotd

The dotfiles manager. Runs all stages in sequence: environment → fileset → packages → symlinks → init.sh.

## Global flags

These flags apply to all `dotd` subcommands:

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `$DOTD_FILES` → `$DOTFILES` → config.yaml `dotfiles` → cwd | Path to dotfiles repo |
| `--dotd-config` | `$DOTD_CONFIG_FILE` → `~/.config/dot-dagger/config.yaml` | Path to config.yaml |
| `--dotd-env` | `$DOTD_ENV_FILE` → `~/.config/dot-dagger/env.yaml` | Path to env.yaml |
| `--env key=value` | — | Override an env key (repeatable) |
| `--dry-run` | false | Print actions without executing |
| `--force` | false | Override safety checks |
| `--log-level` | `info` | Log verbosity: debug, info, warn, error |
| `--debug` | false | Shorthand for `--log-level=debug` |
| `--quiet` | false | Suppress informational logs (data output unaffected) |

Path flags appear in a subcommand's `--help` only when relevant to that command; all of them parse everywhere.

`dotd help --all` shows all commands, including hidden internal helpers.

---

## dotd unapply

Remove symlinks created by `dotd apply`. Re-runs the pipeline to determine the expected link plan, then removes each symlink whose destination points to the expected source file. Also removes init.sh if present.

```sh
dotd unapply                   # preview, then prompt for confirmation
dotd unapply --dry-run         # preview only, no changes
dotd unapply --yes             # skip confirmation prompt
dotd unapply --all             # remove all dotfiles symlinks regardless of @when predicates
```

---

## dotd apply

Full reconciliation: resolves environment, installs packages, applies symlinks, writes init.sh.

`apply` is idempotent and resumable, not transactional. If a run fails partway (for example, a destination is blocked by a non-symlink file), the work already done stays on disk; fix the cause and re-run `dotd apply` to converge to the full plan. There is no rollback and no manual cleanup required.

```sh
dotd apply -f ~/dotfiles
dotd apply -f ~/dotfiles --dry-run
dotd apply -f ~/dotfiles --debug
dotd apply -f ~/dotfiles --env context=work
```

---

## dotd check

Validates all stages without making any changes. Reports the state of packages, symlinks, and the dependency graph.

```sh
dotd check -f ~/dotfiles
dotd check -f ~/dotfiles --debug
```

**Symlink states:**

| State | Meaning |
|---|---|
| `ok` | Symlink exists and points to the right file |
| `missing` | Symlink doesn't exist yet |
| `wrong-target` | Symlink exists but points elsewhere |
| `conflict` | A non-symlink file exists at the destination |

---

## dotd setup

Interactive onboarding. Writes config.yaml and (if absent) env.yaml. Does not touch your dotfiles repo or shell RC file — `dotd init` does that.

```sh
dotd setup
dotd setup --non-interactive  # accept all defaults (alias: -n)
dotd setup -n -f ~/my-dotfiles  # scripted, custom dotfiles directory
```

Sets the dotfiles path, then writes config.yaml and (if absent) env.yaml. Run `dotd init` next to scaffold convention directories in your dotfiles repo.

---

## dotd teardown

Remove dot-dagger system-level configuration from this machine. Removes config.yaml, env.yaml (at the resolved paths — `--dotd-config`, `--dotd-env`, and `DOTD_*` overrides are honored), and the dotd source line from your shell RC file. The preview shows the exact paths before anything is removed. Does not remove symlinks — run `dotd unapply` first.

```sh
dotd teardown                  # preview, then prompt for confirmation
dotd teardown --yes            # skip confirmation prompt
```

---

## dotd init

Scaffold `.dagger` convention directories in your dotfiles repo. Prompts for shell scripts, config files, and bin scripts directories. Creates each directory if absent and writes a `.dagger` file (idempotent). Also offers to append the init.sh source line to your shell RC file — this is the step that wires dotd into your shell (not `setup`).

Requires config.yaml — run `dotd setup` first.

```sh
dotd init
dotd init --non-interactive   # accept all defaults (alias: -n)
```

`dotd setup -n && dotd init -n` scripts a full bootstrap on a fresh machine.

---

## dotd config

Inspect and modify tool configuration (config.yaml).

```sh
dotd config show               # display all config key=value pairs
dotd config get dotfiles       # get a single config value
dotd config set dotfiles ~/dotfiles  # set a config value
dotd config edit               # open config.yaml in $EDITOR
```

**Config keys:**

| Key | Description |
|---|---|
| `dotfiles` | Path to your dotfiles repo |

---

## dotd paths

Print where every anchor and tool path resolves on this machine. Read-only; makes no changes.

```sh
dotd paths          # print resolved paths in human-readable form
dotd paths --json   # machine-readable output
```

Output includes: home (`~`), `$bin`, `$config`, generated files directory, init.sh, dotfiles repo, config.yaml, and env.yaml.

---

## dotd adopt

Moves a file into the dotfiles repo — inferring the destination from the file's name and properties — and replaces the original with a symlink.

```sh
dotd adopt ~/.bashrc                  # → config/dot-bashrc
dotd adopt ~/bin/my-script            # → bin/my-script (marked executable)
dotd adopt ~/.gitconfig --to config/dot-gitconfig-personal
dotd adopt ~/.zshrc --yes             # accept inferred destination without prompting
```

**Inference rules** (checked in order):

| Characteristic | Destination |
|---|---|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` | `shellrc/<name>` |
| Hidden file (`.bashrc`, etc.) | `config/dot-<name>` |
| `.conf`, `.config`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` | `config/<name>` |
| Anything else | Error — use `--to` |

**Flags:**

| Flag | Description |
|---|---|
| `--to <path>` | Destination path relative to dotfiles root (overrides inference) |
| `--yes`, `-y` | Skip the confirmation prompt |

The original file is replaced with a symlink to its new location in the repo. Use `--dry-run` to preview without moving anything.

---

## dotd env

Environment inspection and modification.

```sh
dotd env show                  # print all resolved key=value pairs
dotd env get os                # print a single key's value
dotd env set context work      # write a key to env.yaml
dotd env set context work --dry-run
dotd env diff                  # show keys where env.yaml differs from shell env
```

`dotd env show` is the first thing to reach for when a `@when` condition isn't behaving as expected.

**Auto-detected keys:**

| Key | Detection method |
|---|---|
| `os` | `runtime.GOOS` — `macos` or `linux` |
| `distro` | `ID` from `/etc/os-release` on Linux; `"macos"` on macOS |
| `shell` | `$SHELL` environment variable |

---

## dotd dag

Inspect the dependency graph (load order) used for init.sh.

```sh
dotd dag order                         # print active nodes in dependency order
dotd dag order --env os=macos          # preview for a different environment
dotd dag order --json                  # machine-readable output
```

init.sh itself is written by `dotd apply`. The generated file sources all active scripts in dependency order and contains only `source` calls — no conditions, no loops. All condition evaluation happened at `apply` time.

---

## dotd bundle

Bundle a node and its transitive `@after` dependencies into a single portable shell script. Concatenates dependencies (in DAG order) followed by the target file itself.

```sh
dotd bundle shellrc/my-script.sh                     # print bundle to stdout
dotd bundle shellrc/my-script.sh -o /tmp/bundle.sh   # write to file
dotd bundle shellrc/my-script.sh --include-env       # prepend resolved env as export lines
```

**Flags:**

| Flag | Description |
|---|---|
| `-o, --output <file>` | Write output to file instead of stdout |
| `--include-env` | Prepend resolved env as `export` lines |

---

## dotd package

Package management.

```sh
dotd package generate          # generate shell install script (preview)
dotd package generate | sudo sh  # install packages
dotd package generate > packages-install.sh  # write to file
dotd package check             # report status without installing
dotd package list              # list all declared packages
```

**Package status values:**

| Status | Meaning |
|---|---|
| `installed` | Binary is on `$PATH` |
| `installable` | Not installed, but a manager is available |
| `not available` | Not installed and no manager can install it |
| `MISSING` | Hard requirement (`@require`) that can't be met |

---

## dotd compose

Composition target management. A compose target is a directory with `compose: true` in its `.dagger` file — dotd concatenates the directory's files into a single generated file.

```sh
dotd compose list              # list active compose targets
dotd compose check             # check whether generated files are up to date
```

`dotd apply` automatically regenerates compose targets when they are stale. Use `dotd compose check` to inspect state without making changes.

See [`.dagger` reference](dagger.md#composition) for how to declare a compose target.
