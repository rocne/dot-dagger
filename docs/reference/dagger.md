# .dagger

Per-directory configuration file. Place a `.dagger` file in any directory in your dotfiles repo to control how files in that directory are processed. Settings cascade downward — inner `.dagger` files override outer ones, and `defaults` apply to all files in the directory tree below.

Files that can't carry annotations (JSON, XML, compiled binaries) are declared here instead. Everything a file annotation can express, `.dagger` can express too.

## Format

```yaml
# directory-level conditions and behavior
when: os=macos
link_root: ~/.config/nvim
name: nvim.config
actions:
  - source

# defaults inherited by every file in this directory and its children
defaults:
  when: context=work
  actions:
    - link

# per-file metadata for files without comment syntax
files:
  settings.json:
    when: os=macos
    name: nvim.settings
    actions:
      - link(~/.config/nvim/settings.json)

# enable composition for this directory (see Composition below)
compose: true
# or equivalently:
composition:
  enabled: true

# override convention directory names (root .dagger only)
conventions:
  shellrc: scripts
  bin: bin
  config: dotfiles
```

---

## Top-level fields

These fields apply to the directory that contains the `.dagger` file.

| Field | Type | Description |
|---|---|---|
| `when` | string | Gate traversal of this directory on a condition. If false, no files here are processed. |
| `link_root` | string | Override the symlink root for this directory and all subdirectories. `~/` is expanded to the home directory. |
| `name` | string | Override the directory's logical name. |
| `actions` | list of strings | Actions for this directory node (mainly used with `compose: true`). |
| `after` | list of strings | Load-order dependencies for this directory. |
| `require` | list of strings | Hard package dependencies. |
| `request` | list of strings | Soft package dependencies. |

---

## defaults

Fields under `defaults` cascade to every file in this directory and all subdirectories. A file's own annotations refine but don't replace inherited defaults.

```yaml
defaults:
  when: context=work
  actions:
    - source
```

| Field | Description |
|---|---|
| `when` | ANDed with each file's own `@when` |
| `actions` | Inherited action list — files can override the destination (e.g. override a `link` destination) |
| `link_root` | Inherited symlink root |

**Example — gate an entire directory:**

```yaml
# nosync-work/.dagger
defaults:
  when: context=work
```

All files under `nosync-work/` are only active when `context=work`. No annotation needed on each file.

**Example — source everything in a directory:**

```yaml
# shellrc/.dagger
defaults:
  actions:
    - source
```

This is the `.dagger` that `dotd init` writes into `shellrc/` — it's what makes every file there get sourced into `init.sh`.

---

## files

Per-file metadata for files that can't carry annotations. Each key is a filename (not a path) relative to the directory containing the `.dagger` file.

```yaml
files:
  settings.json:
    when: os=macos
    name: nvim.settings
    actions:
      - link(~/.config/nvim/settings.json)
  dot-gitconfig-work:
    when: context=work
    actions:
      - link(~/.gitconfig)
```

All annotation fields are supported:

| Field | Equivalent annotation |
|---|---|
| `when` | `@when` |
| `name` | `@name` |
| `actions` | `@action` (list of action strings) |
| `after` | `@after` (list of logical names) |
| `require` | `@require` (list of package names) |
| `request` | `@request` (list of package names) |
| `disable` | `@disable` |

---

## Action strings

The `actions` field (top-level, in `defaults`, or in `files`) takes a list of action strings:

| String | Effect |
|---|---|
| `source` | Source this file in `init.sh` |
| `no-source` | Include in dependency graph but do not source in `init.sh` |
| `link` | Symlink this file; destination derived from `link_root` + relative path |
| `link(<dest>)` | Symlink this file to the explicit destination |
| `compose` | Assemble this directory's files into a single generated file |

See [Annotations reference](annotations.md#action) for the full action semantics.

---

## Composition

A directory with `compose: true` (or `composition: { enabled: true }`) is a **compose target**. Files inside are **compose fragments** — dotd concatenates them in dependency order into a single generated file.

The generated filename is derived from the directory name by stripping `nosync-` and `dot-` prefixes and a `.d` suffix:

```
shellrc/dot-zshrc.d/   →  generated file: zshrc
shellrc/nosync-dot-zshrc.d/  →  generated file: zshrc
```

```yaml
# shellrc/dot-zshrc.d/.dagger
compose: true
actions:
  - compose
  - source        # source the generated file in init.sh
```

The `dotd compose list` and `dotd compose check` commands inspect compose targets.

---

## Conventions

The `conventions` key (valid only in the root `.dagger`) renames the three convention directories. By default:

- `shellrc` — shell scripts sourced into `init.sh`
- `bin` — executables symlinked onto `$PATH`
- `config` — config files symlinked into `~/.config`

Override any or all of them:

```yaml
conventions:
  shellrc: scripts
  bin: bin
  config: dotfiles
```

---

## Cascading

Multiple `.dagger` files cascade from root to leaf. For any given file:

- `when` — parent `defaults.when` AND child `defaults.when` AND file's own `@when` are all ANDed together
- `link_root` — nearest ancestor `.dagger` with a non-empty `link_root` wins
- `actions` — accumulated from ancestors; a file annotation can override the destination of an inherited `link`

**Example:**

```
dotfiles/
  .dagger              # (empty)
  config/
    .dagger            # defaults.actions: [link] (link_root defaults to ~/.config)
    nvim/
      .dagger          # link_root: ~/.config/nvim
      init.lua         # → ~/.config/nvim/init.lua
      lua/
        plugins.lua    # → ~/.config/nvim/lua/plugins.lua
```
