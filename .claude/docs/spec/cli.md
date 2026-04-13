# §11, §13 — CLI Interface & Bootstrap

## 11. CLI Interface

```
dotd <command> [options]
```

### Core commands

| Command | Description |
|---------|-------------|
| `dotd install` | Set up dot-dagger — rc wiring, first-run env prompts, gitignore check for `nosync-*` |
| `dotd install --apply` | Set up then immediately apply |
| `dotd apply` | Full reconciliation — evaluate predicates, resolve DAG, symlink, generate `init.sh` |
| `dotd check` | Full status and validation — environment health, filesystem drift, predicate/DAG/annotation errors |
| `dotd add <file>` | Begin tracking a file — fuzzy picker |
| `dotd uninstall <path>` | Remove artifacts for files under a path |
| `dotd uninstall --all` | Remove everything |

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
| `--all` | Required for unscoped destructive operations (e.g. `dotd uninstall --all`) |

Interactivity defaults to `--interactive` when a TTY is detected, `--no-interactive` otherwise.

---

## 13. Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/<user>/dot-dagger/main/bootstrap.sh | sh
```

`bootstrap.sh` downloads the appropriate pre-built binary, prompts for the dotfiles repo URL, clones the repo, and runs `dotd install --apply`.
