# Conditions

A condition is an expression that evaluates to true or false for a given machine. Files annotated with `@when` are only included when their condition is true. Files with no `@when` are always included.

```sh
# @when os=macos                          # only on macOS
# @when os=macos AND context=work         # macOS and work context
# @when os=linux AND exists(brew)         # Linux with Homebrew installed
# @when shell=zsh,bash                    # zsh or bash
```

Conditions are evaluated once when you run `dotd apply`. They are never re-evaluated at shell startup.

## Environment keys

The environment is a flat map of string keys to string values. Most keys are detected automatically from the current machine. Run `dotd env show` to see the full map:

```sh
dotd env show
# os=macos
# shell=zsh
# distro=macos
# context=personal
```

### Built-in keys

| Key | How it's detected | Example values |
|---|---|---|
| `os` | `runtime.GOOS` | `macos`, `linux` |
| `distro` | `ID` from `/etc/os-release` on Linux; `"macos"` on macOS | `ubuntu`, `fedora`, `macos` |
| `shell` | `$SHELL` environment variable | `zsh`, `bash`, `fish` |

### Custom keys

Any key not listed above must be set explicitly in `env.yaml`:

```yaml
env:
  context: personal   # work, personal, or anything you define
  machine: laptop
```

Custom keys can also be set temporarily with `--env` without modifying `env.yaml`:

```sh
dotd apply --env context=work
```

## Writing conditions

### Key-value comparison

```sh
# @when os=macos
# @when context=work
# @when shell=zsh
```

### OR with commas (same key)

A comma within a value means "any of these":

```sh
# @when os=macos,linux        # os is macos or linux
# @when shell=zsh,bash        # shell is zsh or bash
```

### AND / OR operators

```sh
# @when os=macos AND context=work
# @when os=macos OR os=linux
# @when os=macos AND (shell=zsh OR shell=bash)
```

`AND` binds more tightly than `OR`. Use parentheses to change precedence.

### Multiple @when lines

Multiple `@when` lines on the same file are ANDed together:

```sh
# @when os=macos
# @when context=work
# equivalent to: @when os=macos AND context=work
```

### Functions

Conditions can call built-in functions:

```sh
# @when exists(brew)          # brew is on PATH
# @when installed(ripgrep)    # ripgrep binary is on PATH
# @when installable(ripgrep)  # ripgrep is in packages.yaml with an available manager
```

| Function | True when |
|---|---|
| `exists(binary)` | `binary` is found on `$PATH` |
| `installed(pkg)` | the binary for `pkg` is found on `$PATH`; uses `packages.yaml` for binary name resolution |
| `installable(pkg)` | `pkg` has an entry in `packages.yaml` with at least one manager available on `$PATH` |

Functions and comparisons can be mixed freely:

```sh
# @when os=macos AND exists(brew)
# @when shell=zsh AND installed(starship)
```

## Debugging conditions

If a file isn't being included when you expect it to be, check the resolved environment first:

```sh
dotd env show
```

Then try a dry run with debug logging:

```sh
dotd apply --dry-run --debug
```

You can also override a key temporarily to test:

```sh
dotd apply --env os=linux --dry-run
```
