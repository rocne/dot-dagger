# §11, §13 — CLI Interface & Bootstrap

## 11. CLI Interface

```
dotd <command> [options]
```

### Core commands

| Command | Description |
|---------|-------------|
| `dotd install` | Set up dot-dagger — rc wiring, first-run env prompts |
| `dotd install --apply` | Set up then immediately apply |
| `dotd apply` | Full reconciliation — evaluate predicates, resolve DAG, symlink, generate `init.sh` |
| `dotd diff` | Show what apply would change |
| `dotd check` | Validate predicates, DAG, annotations, `.dotd.yaml` files |
| `dotd status` | Full status report |
| `dotd status config` | Config and annotation validation |
| `dotd status env` | Environment health |
| `dotd status files` | Filesystem drift |
| `dotd doctor` | Analyse status, propose and optionally apply fixes |
| `dotd add <file>` | Begin tracking a file — fuzzy picker or `--module` |
| `dotd uninstall <path>` | Remove artifacts for files under a path |
| `dotd uninstall --all` | Remove everything |

### `dotd module` subcommands

| Command | Description |
|---------|-------------|
| `dotd module create <n>` | Scaffold a new directory with `scripts/`, `bin/`, `dots/`, and optional `.dotd.yaml` |
| `dotd module list` | List all directories with their active file counts |

### `dotd env` subcommands

| Command | Description |
|---------|-------------|
| `dotd env list` | Show all env key-value pairs and their sources |
| `dotd env get <key>` | Get a specific key |
| `dotd env set <key=val>` | Set a key in `env.yaml` |

### Global flags

| Flag | Description |
|------|-------------|
| `--force` | Override safety checks |
| `--dry-run` | Print actions without executing. Does not invoke annotation handlers. |
| `--env <key=val>` | Override env key for this invocation |
| `--interactive` / `--no-interactive` | Force interactivity mode |
| `--verbose` | Detailed output |
| `--all` | Required for unscoped destructive operations |

Interactivity defaults to `--interactive` when a TTY is detected, `--no-interactive` otherwise.

---

## 13. Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/<user>/dot-dagger/main/bootstrap.sh | sh
```

`bootstrap.sh` downloads the appropriate pre-built binary, prompts for the dotfiles repo URL, clones the repo, and runs `dotd install --apply`.
