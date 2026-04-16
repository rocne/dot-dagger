# dote

Environment tool. Detects OS, distro, and shell. Loads overrides from `env.yaml`. Produces the resolved environment used for all condition evaluation.

## Commands

### dote show

Print all resolved key=value pairs.

```sh
dote show
dote show --env context=work   # override a key for this run
```

### dote get

Print the value of a single key.

```sh
dote get os
dote get context
```

### dote set

Write a key to `env.yaml`.

```sh
dote set context=work
dote set context=work --dry-run
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-f, --files` | `~/dotfiles` | Path to dotfiles repo |
| `--env-file` | `~/dotfiles/env.yaml` | Path to env.yaml |
| `--env key=value` | — | Override a key (repeatable, not persisted) |
| `--dry-run` | false | Print actions without executing |
| `--verbose` | false | Detailed output |

## Auto-detected keys

| Key | Detection method |
|---|---|
| `os` | `runtime.GOOS` — `macos` or `linux` |
| `distro` | `/etc/os-release` on Linux; `sw_vers` on macOS |
| `shell` | `$SHELL` environment variable |

## Debugging conditions

If a `@when` condition isn't behaving as expected, `dote show` is the first thing to check:

```sh
dote show
# os=macos
# shell=zsh
# distro=sequoia
# context=personal

# Simulate a different environment
dote show --env os=linux --env context=work
```
