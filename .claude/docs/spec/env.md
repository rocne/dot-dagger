# §7 — Config Files

## `env.yaml` (`~/.config/dot-dagger/env.yaml`, not committed — machine local)

Stores the resolved environment and path configuration for this machine.

```yaml
env:
  context: work
  role: desktop

dotfiles_repo: ~/dotfiles

# Path overrides — all optional; see resolution order below.
# link_root: ~
# bin_dir: ~/.local/bin/dot-dagger
# generated_dir: ~/.local/share/dot-dagger/generated
# init_file: ~/.local/share/dot-dagger/init.sh
```

All fields are optional. Keys under `env` override auto-detected values.

### Path fields

| Field | Description | Fallback |
|-------|-------------|---------|
| `dotfiles_repo` | Path to dotfiles repo | `$DOTFILES` env var, then cwd |
| `link_root` | Symlink root for `conf/` files | `$HOME` |
| `bin_dir` | Destination for `bin/` executables | `~/.local/bin/dot-dagger` |
| `generated_dir` | Compose output directory | `$XDG_DATA_HOME/dot-dagger/generated` |
| `init_file` | Generated shell init file | `$XDG_DATA_HOME/dot-dagger/init.sh` |

### Path resolution order

For every managed path, `dotd` resolves in this order — first non-empty wins:

1. CLI flag (e.g. `--link-root`)
2. `DOTD_*` environment variable (e.g. `DOTD_LINK_ROOT`)
3. `env.yaml` field (e.g. `link_root`)
4. XDG/system default

**Exception:** `--env-file` itself cannot be stored in `env.yaml` (circular). Its chain is:
CLI `--env-file` → `DOTD_ENV_FILE` → `$XDG_CONFIG_HOME/dot-dagger/env.yaml`.

### `DOTD_*` environment variables

| Variable | Corresponding flag |
|----------|-------------------|
| `DOTD_ENV_FILE` | `--env-file` |
| `DOTD_FILES` | `--files` (also: `$DOTFILES` legacy) |
| `DOTD_INIT_FILE` | `--init-file` |
| `DOTD_LINK_ROOT` | `--link-root` |
| `DOTD_BIN_DIR` | `--bin-dir` |
| `DOTD_GENERATED_DIR` | `--generated-dir` |

---

## `.dotd.yaml` (per-directory, in dotfiles repo)

The only config file in the dotfiles repo. See [dag.md §6](dag.md) for full structure.

There is no separate `config.yaml` or global schema file — environment keys, annotation handlers, and convention directory names are not configurable at the repo level in the current implementation.

---

See [predicates.md](predicates.md) for env key definitions, resolution precedence, and handling of missing keys.
