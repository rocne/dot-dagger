# dotp

Package management tool. Reads `@require` and `@request` annotations across your dotfiles and installs packages using whichever manager is available on the current machine.

Unlike `dotr package`, `dotp` operates **without condition filtering**. Use `dotp` for unconditional introspection or scripting. Use `dotr package` when you want conditions applied.

## Commands

### dotp install

Install all packages declared in active nodes.

```sh
dotp install -f ~/dotfiles
dotp install -f ~/dotfiles --dry-run
```

### dotp check

Report package status without installing.

```sh
dotp check -f ~/dotfiles
dotp check -f ~/dotfiles --verbose
```

Status values:

| Status | Meaning |
|---|---|
| `installed` | Binary is on `$PATH` |
| `installable` | Not installed, but a manager is available |
| `not available` | Not installed and no manager can install it |
| `MISSING` | Hard requirement (`@require`) that can't be met |

### dotp list

List all packages declared across active nodes, with their source files.

```sh
dotp list -f ~/dotfiles
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `~/dotfiles` | Path to dotfiles repo |
| `--env-file` | `~/dotfiles/env.yaml` | Path to env.yaml |
| `--dry-run` | false | Print actions without executing |
| `--verbose` | false | Detailed output |

## How packages are installed

When `dotp` encounters a `@require` or `@request` annotation, it:

1. Looks up the package in `packages.yaml`
2. Checks whether the package is already installed (binary on `$PATH`, or custom `check` command)
3. If not installed, finds the first available package manager from the `prefer` list (or global `priority`)
4. Runs the install command

For `@require`: if the package can't be installed and isn't present, `dotp` exits with an error.

For `@request`: if the package can't be installed, it's silently skipped.

See [packages.yaml](packages-yaml.md) for how to define packages and managers.
