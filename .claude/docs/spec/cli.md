# §11, §13 — CLI Interface & Bootstrap

## 11. CLI Interface

```
dotd <command> [options]
```

### Top-level commands

| Command | Description |
|---------|-------------|
| `dotd apply` | Full reconciliation — env → fileset → packages → links → init.sh |
| `dotd check` | Validate all stages without making changes |
| `dotd setup` | Interactive onboarding — scaffold dotfiles repo structure and config files |
| `dotd adopt <file>` | Import a file into the dotfiles repo, inferring the destination directory |
| `dotd completion <shell>` | Generate shell completion script (bash, zsh, fish, powershell) |

### `dotd link` subcommands

| Command | Description |
|---------|-------------|
| `dotd link apply` | Plan and apply symlinks for active `conf/` and `bin/` nodes |
| `dotd link check` | Report symlink state without making changes |
| `dotd link remove` | Remove owned symlinks |

### `dotd dag` subcommands

| Command | Description |
|---------|-------------|
| `dotd dag apply` | Resolve DAG and write `init.sh` |
| `dotd dag check` | Validate DAG ordering without writing `init.sh` |

### `dotd env` subcommands

| Command | Description |
|---------|-------------|
| `dotd env show` | Display all resolved env key-value pairs |
| `dotd env get <key>` | Get a specific key |
| `dotd env set <key=value>` | Set a key in `env.yaml` |
| `dotd env diff` | Show keys where `env.yaml` overrides auto-detected values |

### `dotd package` subcommands

| Command | Description |
|---------|-------------|
| `dotd package check` | Report package status without installing |
| `dotd package list` | List all packages declared in active nodes |
| `dotd package generate` | Generate a shell install script for active package requirements |

### Global flags

| Flag | Description |
|------|-------------|
| `--files <path>` | Path to dotfiles repo (default: auto-detected) |
| `--env-file <path>` | Path to `env.yaml` (default: `~/.config/dot-dagger/env.yaml`) |
| `--env <key=value>` | Override env key for this invocation (repeatable) |
| `--init-file <path>` | Path to write `init.sh` |
| `--link-root <path>` | Symlink root for `conf/` files (default: `$HOME`) |
| `--bin-dir <path>` | Bin directory for `bin/` files |
| `--dry-run` | Print actions without executing |
| `--force` | Override safety checks |
| `--verbose` | Detailed output |

---

## 13. Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

`install.sh` downloads the appropriate pre-built binary for the current platform and places it on `PATH`.
