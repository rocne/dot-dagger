# §11, §13 — CLI Interface & Bootstrap

## 11. CLI Interface

```
dotd <command> [options]
```

### Top-level commands

| Command | Description |
|---------|-------------|
| `dotd apply` | Full reconciliation — env → walk → filter → order → act → init.sh |
| `dotd check` | Validate all stages without making changes |
| `dotd init` | Interactive onboarding — scaffold dotfiles repo structure and config files |
| `dotd adopt <file>` | Import a file into the dotfiles repo _(not yet migrated to v2)_ |
| `dotd completion <shell>` | Generate shell completion script (bash, zsh, fish, powershell) |

### `dotd list` subcommands

| Command | Description |
|---------|-------------|
| `dotd list` | List active nodes (logical name, actions, path) |
| `dotd list --inactive` | List all nodes including inactive, with conditions |
| `dotd list --json` | Machine-readable JSON output |

### `dotd dag` subcommands

| Command | Description |
|---------|-------------|
| `dotd dag check` | Print nodes in dependency order |

### `dotd env` subcommands

| Command | Description |
|---------|-------------|
| `dotd env show` | Display all resolved env key=value pairs |
| `dotd env get <key>` | Get a specific key |
| `dotd env set <key> <value>` | Set a key in `env.yaml` |
| `dotd env diff` | Show keys where `env.yaml` overrides shell-detected values |
| `dotd env edit` | Open `env.yaml` in `$EDITOR` |

### `dotd compose` subcommands

| Command | Description |
|---------|-------------|
| `dotd compose list` | List active compose targets |
| `dotd compose check` | Check compose targets for staleness |

Note: compose files are generated (and symlinked) by `dotd apply`.

### `dotd package` subcommands

| Command | Description |
|---------|-------------|
| `dotd package list` | List all packages referenced in active nodes |
| `dotd package check` | Report install status for all referenced packages |
| `dotd package generate` | Generate a shell install script for active package requirements |

### `dotd config` subcommands

| Command | Description |
|---------|-------------|
| `dotd config show` | Show current config.yaml values |
| `dotd config get <key>` | Get a single config key |
| `dotd config set <key> <value>` | Set a key in config.yaml |
| `dotd config edit` | Open config.yaml in `$EDITOR` |

### Global flags

All path flags resolve via: CLI arg → `DOTD_*` env var → `env.yaml` field → XDG/system default.
See [env.md §7](env.md) for the full resolution table.

| Flag | Description |
|------|-------------|
| `--files <path>` | Path to dotfiles repo (`DOTD_FILES` / `$DOTFILES` / `dotfiles_repo` in env.yaml / cwd) |
| `--env-file <path>` | Path to `env.yaml` (`DOTD_ENV_FILE` / `$XDG_CONFIG_HOME/dot-dagger/env.yaml`) |
| `--env <key=value>` | Override env key for this invocation (repeatable; highest precedence) |
| `--init-file <path>` | Path to write `init.sh` (`DOTD_INIT_FILE` / `init_file` in env.yaml / XDG data default) |
| `--link-root <path>` | Symlink root for `conf/` files (`DOTD_LINK_ROOT` / `link_root` in env.yaml / `$HOME`) |
| `--bin-dir <path>` | Bin directory for `bin/` files (`DOTD_BIN_DIR` / `bin_dir` in env.yaml / `~/.local/bin/dot-dagger`) |
| `--generated-dir <path>` | Path to write composed files (`DOTD_GENERATED_DIR` / `generated_dir` in env.yaml / XDG data default) |
| `--dry-run` | Print actions without executing |
| `--force` | Override safety checks |
| `--verbose` | Detailed output |

---

## 13. Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
```

`install.sh` downloads the appropriate pre-built binary for the current platform and places it on `PATH`.
