# Annotation reference

Annotations are comments placed at the top of a file. Scanning begins at the first line (skipping a shebang if present) and stops at the first blank line or non-comment line.

Shell files use `#`. C-style files use `//`. Files without supported comment syntax use [`.dagger`](dagger.md).

```sh
#!/bin/bash
# @when(os=macos AND context=work)
# @after(shellrc/base/)
# @require(ripgrep)
# @action(no-source)

export EDITOR=nvim
```

---

## @when

Gates file inclusion on a condition. A file with no `@when` is always active. Multiple `@when` lines are ANDed together.

```sh
# @when(os=macos)
# @when(os=macos AND context=work)
# @when(os=macos,linux)               # comma = OR shorthand for the same key
# @when(shell=zsh AND exists(brew))
# @when(os=macos AND (shell=zsh OR shell=bash))
```

See [Conditions](../concepts/conditions.md) for the full expression syntax.

---

## @name

Overrides the file's logical name. The default logical name is derived from the path by stripping `nosync-`/`dot-` prefixes and extensions.

```sh
# @name(shellrc.aliases)
```

Primary use: **variant files** — two files representing the same thing under mutually exclusive conditions:

```sh
# shellrc/aliases-macos.sh
# @name(shellrc.aliases)
# @when(os=macos)

# shellrc/aliases-linux.sh
# @name(shellrc.aliases)
# @when(os=linux)
```

Other files can then `@after(shellrc.aliases)` to depend on whichever variant is active. Two active files with the same logical name is an error.

---

## @after

Declares a load-order dependency. Only meaningful for files that are sourced into `init.sh`. Controls the order scripts appear in `init.sh`.

```sh
# @after(shellrc/base/)       # all active files under shellrc/base/
# @after(shellrc/env/)        # all active files under shellrc/env/
# @after(shellrc.helpers)     # one specific file, by logical name
```

- A path ending in `/` expands to all active files under that path
- A reference without `/` is matched against logical names
- If no matching files are active, the dependency is silently ignored

Files with no `@after` are ordered alphabetically by logical name within their position in the dependency graph.

---

## @action

The unified action annotation. Declares what dotd does with this file. Multiple `@action` lines can appear; they are additive (with the exception that `no-source` cancels `source`).

```sh
# @action(source)                   # source in init.sh
# @action(no-source)                # keep in graph but don't source
# @action(link(~/.gitconfig))       # symlink to explicit path
# @action(link)                     # symlink; destination derived from link_root
```

### Action types

| Action | Effect |
|---|---|
| `source` | Source this file in `init.sh`. Default for files in `shellrc/`. |
| `no-source` | Include in load-order graph so others can `@after` it, but exclude from `init.sh`. |
| `link(<dest>)` | Symlink this file to the explicit destination. Absolute paths are used as-is; `~/` is expanded to home. |
| `link` | Symlink this file; destination derived from the directory's `link_root` + relative path. Default for files in `config/` and `bin/`. |
| `compose` | Assemble this directory's files into a single generated file. Only valid on directories with `compose: true` in `.dagger`. |

### Aliases

These shorthand annotations normalize to `@action` internally:

| Shorthand | Equivalent |
|---|---|
| `@source` | `@action(source)` |
| `@no-source` | `@action(no-source)` |
| `@link(<dest>)` | `@action(link(<dest>))` |
| `@symlink(<dest>)` | `@action(link(<dest>))` (legacy spelling) |

---

## @link

Symlinks the file to an explicit path. Alias for `@action(link(<dest>))`.

```sh
# @link(~/.gitconfig)
# @link(~/.config/nvim/init.lua)
```

Usually unnecessary — files in `config/` and `bin/` are symlinked automatically via inherited `.dagger` defaults. Use `@link` to override the destination or to symlink a file that lives outside those directories.

`@symlink` is accepted as a legacy spelling of `@link`.

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
# @require(ripgrep)
# @require(fzf)
```

See [dotd package](dotd.md#dotd-package) and [packages.yaml](packages-yaml.md) for how packages are defined and installed.

---

## @request

Declares a soft package dependency. The file is always active. `dotd` installs the package if it can, silently skips it if not.

```sh
# @request(fzf)
# @request(starship)
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

Keeps the file in the dependency graph so other files can `@after` it, but excludes it from `init.sh`. Alias for `@action(no-source)`.

```sh
# shellrc/helpers.sh
# @no-source
```

Use this when a file defines shared state or functions that other scripts depend on, but shouldn't itself be sourced at shell startup.

---

## @source

Forces a file into `init.sh` sourcing even if it isn't in `shellrc/`. Works with any file anywhere in your dotfiles repo. Alias for `@action(source)`.

```sh
# config/shell/extras.sh
# @source
```

Use `@source` when a shell script also needs to be symlinked as a config file, or when the `shellrc/` convention doesn't fit for a particular file.

---

## Sourcing control summary

| Annotation | In load order? | Symlinked? | Sourced in init.sh? |
|---|---|---|---|
| _(none)_ in `shellrc/` | Yes | No | Yes |
| _(none)_ in `config/` | No | Yes | No |
| `@no-source` | Yes | As normal | **No** |
| `@source` | Yes | As normal | **Yes** |
| `@disable` | **No** | **No** | **No** |

---

## .dagger equivalents

For files that can't carry annotations, use `.dagger` in the same directory:

```yaml
files:
  settings.json:
    when: os=macos
    name: app.settings
    actions:
      - link(~/.config/app/settings.json)
    after:
      - shellrc/base/
    require:
      - ripgrep
    disable: false
```

See [.dagger reference](dagger.md) for the full format.
