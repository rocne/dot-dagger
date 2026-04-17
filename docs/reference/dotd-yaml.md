# .dotd.yaml

Per-directory configuration for files that can't carry annotations — JSON, XML, compiled binaries, and anything else without a comment syntax dotd recognizes.

It can appear in any directory in your dotfiles repo. Settings in a `.dotd.yaml` apply to that directory and cascade downward; inner files override outer ones.

## Format

```yaml
# dotd: controls file inclusion and sourcing
dotd:
  when: "os=macos"          # skip this entire directory unless condition is true
  defaults:
    when: "context=work"    # AND with each file's own @when
  files:
    - path: dot-gitconfig   # filename on disk
      when: "context=personal"
      symlink: ~/.gitconfig
      name: ""
      after: ""
      retain_prefix: false
      disable: false
      no_source: false
      source: false

# link: symlink root override for this directory and its children
link:
  link_root: ~/.config/nvim
```

## dotd section

### when

Gates traversal of the entire directory. If the condition is false, no files in this directory (or its subdirectories) are processed.

```yaml
dotd:
  when: "os=macos"
```

### defaults.when

A condition that is ANDed with the `@when` of every file in this directory. Useful for gating an entire subdirectory without adding annotations to each file.

```yaml
dotd:
  defaults:
    when: "context=work"
```

### files

Per-file metadata for files that can't carry annotations. Each entry corresponds to one file in the same directory.

```yaml
dotd:
  files:
    - path: settings.json          # filename on disk (required)
      when: "os=macos"             # equivalent to @when
      symlink: ~/.config/app/settings.json  # equivalent to @symlink
      name: conf.app.settings      # equivalent to @name
      after: scripts/base/         # equivalent to @after
      retain_prefix: true          # equivalent to @retain-prefix
      disable: true                # equivalent to @disable
      no_source: true              # equivalent to @no-source
      source: true                 # equivalent to @source
```

All fields are optional. Omitted fields use their defaults (empty/false).

## link section

### link_root

Overrides the symlink root for this directory and all subdirectories. Paths starting with `~/` are expanded to the home directory at walk time.

```yaml
link:
  link_root: ~/.config/nvim
```

This means all `conf/` files under this directory will be symlinked relative to `~/.config/nvim` instead of `$HOME`.

**Example:**

```
conf/dot-config/nvim/
  .dotd.yaml   ← link.link_root: ~/.config/nvim
  init.lua     → ~/.config/nvim/init.lua
  lua/
    plugins.lua  → ~/.config/nvim/lua/plugins.lua
```

## Cascading

`.dotd.yaml` files cascade: inner directories override outer ones. If `conf/.dotd.yaml` sets `link.link_root: ~/.config` and `conf/nvim/.dotd.yaml` sets `link.link_root: ~/.config/nvim`, the nvim directory uses `~/.config/nvim`.

`dotd.defaults.when` cascades by ANDing: if the parent directory has `defaults.when: "os=macos"` and the child has `defaults.when: "context=work"`, files in the child see an effective when of `(os=macos) AND (context=work)`.
