# dotr

The orchestrator. Runs all four stages in sequence: environment → packages → symlinks → init.sh.

## Global flags

These flags apply to all `dotr` subcommands:

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `~/dotfiles` | Path to dotfiles repo |
| `--env-file` | `~/dotfiles/env.yaml` | Path to env.yaml |
| `--env key=value` | — | Override a key (repeatable) |
| `--init-file` | `~/.local/share/dot-dagger/init.sh` | Path to write init.sh |
| `--link-root` | `$HOME` | Symlink root for conf/ files |
| `--bin-dir` | `~/.local/bin` | Bin directory for bin/ files |
| `--dry-run` | false | Print actions without executing |
| `--force` | false | Override safety checks |
| `--verbose` | false | Detailed output |

---

## dotr apply

Full reconciliation: resolves environment, installs packages, applies symlinks, writes init.sh.

```sh
dotr apply -f ~/dotfiles
dotr apply -f ~/dotfiles --dry-run
dotr apply -f ~/dotfiles --verbose
dotr apply -f ~/dotfiles --env context=work
```

---

## dotr check

Validates all stages without making any changes. Reports the state of packages, symlinks, and the dependency graph.

```sh
dotr check -f ~/dotfiles
dotr check -f ~/dotfiles --verbose
```

---

## dotr setup

Interactive onboarding. Scaffolds a dotfiles repo, writes config files, and wires up your shell.

```sh
dotr setup
dotr setup --yes              # non-interactive, accept all defaults
dotr setup -f ~/my-dotfiles   # custom dotfiles directory
```

Steps through: dotfiles directory, env.yaml path, init.sh output path, and package managers to pre-populate in packages.yaml. After scaffolding, detects your shell config file and offers to append the source line.

---

## dotr adopt

Copies a file into the dotfiles repo, inferring the destination from the file's name and properties.

```sh
dotr adopt ~/.bashrc                  # → conf/dot-bashrc
dotr adopt ~/bin/my-script            # → bin/my-script (marked executable)
dotr adopt ~/.gitconfig --to conf/dot-gitconfig-personal
dotr adopt ~/.zshrc --yes             # accept inferred destination without prompting
```

**Inference rules** (checked in order):

| Characteristic | Destination |
|---|---|
| Marked executable (`chmod +x`) | `bin/<name>` |
| `.sh`, `.bash`, `.zsh`, `.fish` | `scripts/<name>` |
| Hidden file (`.bashrc`, etc.) | `conf/dot-<name>` |
| `.conf`, `.toml`, `.yaml`, `.yml`, `.ini`, `.cfg`, `.json` | `conf/<name>` |
| Anything else | Error — use `--to` |

**Flags:**

| Flag | Description |
|---|---|
| `--to <path>` | Destination path relative to dotfiles root |
| `--yes`, `-y` | Non-interactive: accept inferred destination |
| `--no-interactive` | Same as `--yes` |

After copying, `adopt` offers to remove the original (interactive mode only). Run `dotr apply` afterwards to create the symlink.

---

## dotr env

Environment inspection and modification. Equivalent to `dote`, but applies conditions from the current machine context.

```sh
dotr env show                  # print all resolved key=value pairs
dotr env get os                # print a single key's value
dotr env set context=work      # write a key to env.yaml
dotr env set context=work --dry-run
```

---

## dotr dag

Dependency graph (load order) management for init.sh. Equivalent to `dotd`, but applies conditions.

```sh
dotr dag apply                         # write init.sh
dotr dag apply --dry-run               # preview without writing
dotr dag apply --init-file ~/init.sh   # custom output path
dotr dag check                         # validate without writing
dotr dag check --verbose               # show numbered load order
```

---

## dotr link

Symlink management. Equivalent to `dotl`, but applies conditions.

```sh
dotr link apply                # create/update symlinks
dotr link apply --dry-run      # preview
dotr link apply --force        # overwrite conflicting files
dotr link check                # report state
dotr link check --verbose      # include ok symlinks in output
dotr link remove               # remove owned symlinks
dotr link remove --dry-run     # preview removal
```

---

## dotr package

Package management. Equivalent to `dotp`, but applies conditions.

```sh
dotr package install           # install declared packages
dotr package install --dry-run # preview
dotr package check             # report status without installing
dotr package list              # list all declared packages
```

---

## Standalone vs subcommand

`dotr` subcommands evaluate `@when` conditions first — only files active on this machine are processed. The standalone tools (`dotd`, `dotl`, `dotp`) operate without condition filtering, which is useful for introspection or scripting.

| Task | With conditions | Without conditions |
|---|---|---|
| Environment | `dotr env show` | `dote show` |
| Load order | `dotr dag apply` | `dotd apply` |
| Symlinks | `dotr link apply` | `dotl apply` |
| Packages | `dotr package install` | `dotp install` |
