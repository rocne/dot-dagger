# Annotation reference

Annotations are comments placed at the top of a file. Scanning begins at the first line (skipping a shebang if present) and stops at the first blank line or non-comment line.

Shell files use `#`. C-style files use `//`. Files without supported comment syntax use [`.dot-dagger.yaml`](dot-dagger-yaml.md).

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep
# @no-source

export EDITOR=nvim
```

---

## @when

Gates file inclusion on a condition. A file with no `@when` is always active. Multiple `@when` lines are ANDed together.

```sh
# @when os=macos
# @when os=macos AND context=work
# @when os=macos,linux               # comma = OR shorthand for the same key
# @when shell=zsh AND exists(brew)
# @when os=macos AND (shell=zsh OR shell=bash)
```

See [Conditions](../concepts/conditions.md) for the full expression syntax.

---

## @name

Overrides the file's logical name. The default logical name is derived from the path by stripping `nosync-`/`dot-` prefixes and extensions.

```sh
# @name scripts.aliases
```

Primary use: **variant files** — two files representing the same thing under mutually exclusive conditions:

```sh
# scripts/aliases-macos.sh
# @name scripts.aliases
# @when os=macos

# scripts/aliases-linux.sh
# @name scripts.aliases
# @when os=linux
```

Other files can then `@after scripts.aliases` to depend on whichever variant is active. Two active files with the same logical name is an error.

---

## @after

Declares a load-order dependency. Only meaningful in `scripts/`. Controls the order scripts appear in `init.sh`.

```sh
# @after scripts/base/        # all active files under scripts/base/
# @after scripts/env/         # all active files under scripts/env/
# @after scripts.helpers      # one specific file, by logical name
```

- A path ending in `/` expands to all active files under that path
- A reference without `/` is matched against logical names
- If no matching files are active, the dependency is silently ignored

Files with no `@after` are ordered alphabetically by logical name within their position in the dependency graph.

---

## @symlink

Symlinks the file to an explicit path instead of the conventional destination derived from its location in `conf/` or `bin/`. Absolute paths are used as-is. Relative paths resolve against the effective `link_root` for the directory.

```sh
# @symlink ~/.gitconfig
# @symlink ~/.config/nvim/init.lua
```

Usually unnecessary — files in `conf/` and `bin/` are symlinked automatically. Use `@symlink` to override the destination or to symlink a file that lives outside those directories.

---

## @retain-prefix

By default, both `dot-` and `nosync-` are stripped from path components when computing logical names and symlink destinations. `@retain-prefix` opts out of this stripping for the **filename** — both prefixes are kept in the destination name.

```sh
# conf/dot-tmux.conf   →  normally: ~/.tmux.conf
# @retain-prefix
# conf/dot-tmux.conf   →  with @retain-prefix: ~/.dot-tmux.conf
```

Directory components above the filename are always stripped regardless of `@retain-prefix`.

---

## @require

Declares a hard package dependency. The file is only active if the package is installed or can be installed. If it can be installed, `dotd` installs it automatically. If it can't and isn't already present, `dotd` errors.

```sh
# @require ripgrep
# @require fzf
```

See [dotd package](dotd.md#dotd-package) and [packages.yaml](packages-yaml.md) for how packages are defined and installed.

---

## @request

Declares a soft package dependency. The file is always active. `dotd` installs the package if it can, silently skips it if not.

```sh
# @request fzf
# @request starship
```

Use `@require` when the file won't work without the package. Use `@request` when the file is useful without it, or when the package is optional.

---

## @disable

Removes the file from all processing: no symlinks, no load order, no sourcing, no package checks. Equivalent to the file not existing.

```sh
# @disable
```

Useful for keeping files in the repo (for reference, backup, or future use) without having them take effect.

---

## @no-source

Keeps the file in the dependency graph so other files can `@after` it, but excludes it from `init.sh`. Only meaningful for files that would otherwise be sourced (files in `scripts/`, or files with `@source`).

```sh
# scripts/helpers.sh
# @no-source
```

Use this when a file defines shared state or functions that other scripts depend on, but shouldn't itself be sourced at shell startup.

---

## @source

Forces a file into `init.sh` sourcing even if it isn't in `scripts/`. Works with any file anywhere in your dotfiles repo.

```sh
# conf/dot-config/shell/extras.sh
# @source
```

Use `@source` when a shell script also needs to be symlinked as a config file, or when the `scripts/` convention doesn't fit for a particular file.

---

## Sourcing control summary

| Annotation | In load order? | Symlinked? | Sourced in init.sh? |
|---|---|---|---|
| _(none)_ in `scripts/` | Yes | No | Yes |
| _(none)_ in `conf/` | No | Yes | No |
| `@no-source` | Yes | As normal | **No** |
| `@source` | Yes | As normal | **Yes** |
| `@disable` | **No** | **No** | **No** |

---

## .dot-dagger.yaml equivalents

For files that can't carry annotations, use `.dot-dagger.yaml` in the same directory:

```yaml
dotd:
  files:
    - path: settings.json
      when: "os=macos"
      disable: false
      no_source: false
      source: false
      retain_prefix: false
      after: ""
      name: ""
      symlink: ""
```

See [.dot-dagger.yaml reference](dot-dagger-yaml.md) for the full format.
