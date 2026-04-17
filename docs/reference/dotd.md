# dotd

The dotfiles manager. Runs all stages in sequence: environment → fileset → packages → symlinks → init.sh.

## Global flags

These flags apply to all `dotd` subcommands:

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `$DOTFILES` or `./` | Path to dotfiles repo |
| `--env-file` | `~/.config/dot-dagger/env.yaml` | Path to env.yaml |
| `--env key=value` | — | Override a key (repeatable) |
| `--init-file` | `~/.local/share/dot-dagger/init.sh` | Path to write init.sh |
| `--link-root` | `$HOME` | Symlink root for conf/ files |
| `--bin-dir` | — | Bin directory for bin/ files |
| `--dry-run` | false | Print actions without executing |
| `--force` | false | Override safety checks |
| `--verbose` | false | Detailed output |

---

## dotd apply

Full reconciliation: resolves environment, installs packages, applies symlinks, writes init.sh.

```sh
dotd apply -f ~/dotfiles
dotd apply -f ~/dotfiles --dry-run
dotd apply -f ~/dotfiles --verbose
dotd apply -f ~/dotfiles --env context=work
```

---

## dotd check

Validates all stages without making any changes. Reports the state of packages, symlinks, and the dependency graph.

```sh
dotd check -f ~/dotfiles
dotd check -f ~/dotfiles --verbose
```

---

## dotd setup

Interactive onboarding. Scaffolds a dotfiles repo, writes config files, and wires up your shell.

```sh
dotd setup
dotd setup --yes              # non-interactive, accept all defaults
dotd setup -f ~/my-dotfiles   # custom dotfiles directory
```

Steps through: dotfiles directory, env.yaml path, init.sh output path, and package managers to pre-populate in packages.yaml. After scaffolding, detects your shell config file and offers to append the source line.

---

## dotd adopt

Copies a file into the dotfiles repo, inferring the destination from the file's name and properties.

```sh
dotd adopt ~/.bashrc                  # → conf/dot-bashrc
dotd adopt ~/bin/my-script            # → bin/my-script (marked executable)
dotd adopt ~/.gitconfig --to conf/dot-gitconfig-personal
dotd adopt ~/.zshrc --yes             # accept inferred destination without prompting
```

**Inference rules** (checked in order):

| Characteristic | Destination |
|---|---|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` | `scripts/<name>` |
| Hidden file (`.bashrc`, etc.) | `conf/dot-<name>` |
| `.conf`, `.config`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` | `conf/<name>` |
| Anything else | Error — use `--to` |

**Flags:**

| Flag | Description |
|---|---|
| `--to <path>` | Destination path relative to dotfiles root |
| `--yes`, `-y` | Non-interactive: accept inferred destination |
| `--no-interactive` | Same as `--yes` |

After copying, `adopt` offers to remove the original (interactive mode only). Run `dotd apply` afterwards to create the symlink.

---

## dotd env

Environment inspection and modification.

```sh
dotd env show                  # print all resolved key=value pairs
dotd env get os                # print a single key's value
dotd env set context=work      # write a key to env.yaml
dotd env set context=work --dry-run
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

Dependency graph (load order) management for init.sh.

```sh
dotd dag apply                         # write init.sh
dotd dag apply --dry-run               # preview without writing
dotd dag apply --init-file ~/init.sh   # custom output path
dotd dag check                         # validate without writing
dotd dag check --verbose               # show numbered load order
```

The generated `init.sh` sources all active scripts in dependency order. It contains only `source` calls — no conditions, no loops. All condition evaluation happened at `apply` time.

---

## dotd link

Symlink management.

```sh
dotd link apply                # create/update symlinks
dotd link apply --dry-run      # preview
dotd link apply --force        # overwrite conflicting files
dotd link check                # report state
dotd link check --verbose      # include ok symlinks in output
dotd link remove               # remove owned symlinks
dotd link remove --dry-run     # preview removal
```

**Symlink states:**

| State | Meaning |
|---|---|
| `ok` | Symlink exists and points to the right file |
| `missing` | Symlink doesn't exist yet |
| `wrong-target` | Symlink exists but points elsewhere |
| `conflict` | A non-symlink file exists at the destination |

---

## dotd package

Package management.

```sh
dotd package generate          # generate shell install script (preview)
dotd package generate | sudo sh  # install packages
dotd package generate -o packages-install.sh  # write to file
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
