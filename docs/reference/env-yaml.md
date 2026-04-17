# env.yaml

Declares your environment context. Most keys (`os`, `shell`, `distro`) are detected automatically — `env.yaml` is for keys that can't be auto-detected, primarily `context`.

## Location

dotd looks for `env.yaml` in two places, in order:

1. `<dotfiles>/env.yaml` (inside your dotfiles repo)
2. `~/.config/dot-dagger/env.yaml`

Override with `--env-file`:

```sh
dotd apply --env-file ~/.config/dot-dagger/env.yaml
```

## Format

```yaml
env:
  context: personal
```

Any key in the `env` map is added to the resolved environment. If a key matches an auto-detected key (`os`, `shell`, `distro`), the manually set value takes precedence.

## Auto-detected keys

These are set automatically and don't need to be in `env.yaml` unless you want to override them:

| Key | Detection method |
|---|---|
| `os` | `runtime.GOOS` — `macos` or `linux` |
| `distro` | `ID` from `/etc/os-release` on Linux; `"macos"` on macOS |
| `shell` | `$SHELL` environment variable |

## Custom keys

Any key you want to use in `@when` conditions that isn't auto-detected must be set here:

```yaml
env:
  context: personal   # used in @when context=work
  machine: laptop     # used in @when machine=laptop
  team: platform      # used in @when team=platform
```

## Overriding at runtime

Use `--env` to override keys for a single run without modifying `env.yaml`:

```sh
# Test what would happen in work context
dotd apply --env context=work --dry-run

# Test on a simulated Linux machine
dotd apply --env os=linux --dry-run
```

## Viewing the resolved environment

```sh
dotd env show
```

Prints all resolved key=value pairs, including auto-detected keys and any overrides from `env.yaml`.
