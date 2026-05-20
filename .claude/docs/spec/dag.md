# §3, §5, §6 — Logical Names, Annotations & .dagger

## 3. Logical Names and the DAG

Every file is a node in the DAG, identified by its logical name — a dot-separated path from the dotfiles repo root with the file extension stripped.

### Name derivation

Applied per path component, in order:
1. Strip leading `nosync-`
2. Strip leading `dot-` (after nosync-)
3. Strip file extension from the final component only

```
tmux/shellrc/helpers.sh         → tmux.shellrc.helpers
shellrc/math.sh                 → shellrc.math
nosync-work/shellrc/aliases.sh  → work.shellrc.aliases
conf/dot-config/tmux/tmux.conf  → conf.config.tmux.tmux
nosync-dot-secrets/api.sh       → secrets.api
```

The logical name always reflects the full path. No segments are skipped regardless of whether intermediate directories have `.dagger` files.

### `@name` — aliasing

`@name` replaces the entire logical name, not just the last segment. It creates a canonical identity for the file independent of its filesystem location:

```bash
# tmux/shellrc/tmux-helpers-macos.sh
# @name tmux.shellrc.helpers
```

This is the primary tool for variant files — two files that represent the same logical unit under different conditions both declare the same `@name`. The resolver enforces that exactly one is active at a time. Predicates must be mutually exclusive.

### `@after` references

`@after` accepts either a full logical name or a path prefix ending in `/`:

```bash
# @after tmux.shellrc.helpers   — specific file by logical name
# @after tmux/                  — all active files under tmux/
# @after tmux/shellrc/          — all active files under tmux/shellrc/
```

Path-prefix references expand to the set of active files under that path. If no files under that path are active, the dependency is a no-op — never an error.

`@after` is purely an ordering constraint. It does not affect inclusion. A file being inactive does not cause an `@after` referencing it to fail.

### Default ordering

Files with no `@after` declarations are sourced in deterministic alphabetical order by logical name within each topological frontier. This ensures reproducible `init.sh` generation regardless of filesystem discovery order.

### Conflict detection

Two active nodes with the same logical name — whether derived or declared via `@name` — is a conflict error. Predicates must be mutually exclusive for variant files sharing a name.

Two active files both declaring `@symlink` to the same destination is a separate symlink destination conflict — detected independently from logical name conflicts. Conventional `conf/` and `bin/` destination conflicts are detected by the linker, which knows the runtime home directory.

---

## 5. Annotations

Metadata is declared as comment annotations at the top of a file.

### Scanning rules

1. If the first line is a shebang (`#!`), skip it
2. Read lines starting with `#` or `//`
3. Non-`@` comment lines are ignored without stopping the scan
4. Blank lines are allowed anywhere in the annotation block without stopping the scan
5. The first non-comment, non-blank line stops the scan

### Supported annotations

| Annotation | Purpose |
|-----------|---------|
| `@when <expr>` | Inclusion predicate. Multiple lines are ANDed. |
| `@after <ref>` | DAG ordering dependency — logical name or path prefix ending in `/` |
| `@name <logical-name>` | Override the full logical name of this file |
| `@action <type>` | Declare what to do with this file. Multiple lines applied in order. See [actions.md](actions.md). |
| `@disable` | Exclude this file from all processing — no DAG, no sourcing, no symlinking |
| `@require <pkg>` | Declare a hard package dependency for this file |
| `@request <pkg>` | Declare a soft (optional) package dependency for this file |

The following annotations are **aliases** for specific `@action` types and remain fully supported:

| Alias | Equivalent |
|-------|-----------|
| `@source` | `@action source` |
| `@no-source` | `@action no-source` |
| `@symlink <dest>` | `@action link(<dest>)` |

### `@action`

Declares what to do with a file. See [actions.md](actions.md) for available action types, sequencing rules, and how explicit actions interact with convention dir defaults.

---

## 6. `.dagger`

An optional metadata file that can appear in any directory. Its primary purpose is to provide metadata for files that cannot carry annotations — JSON, binary, XML, Lua, and other formats without supported comment syntax. Every annotation supported in file headers is also supported here.

All fields are at the top level — no nesting under `dotd:` or `link:` sections.

### Directory-level fields

| Field | Purpose |
|-------|---------|
| `when` | Gates the directory's files — ANDed into each file's effective predicate. Cascades to subdirectories. |
| `link_root` | Overrides the symlink destination root for this subtree. Cascades to subdirectories unless overridden closer. |
| `actions` | Actions for this directory node (e.g. `[compose, link(~/.tmux.conf)]`). |
| `name` | Logical name override for a compose target directory. |
| `composition.enabled` | Marks this directory as a compose target (`true`/`false`). Alias: `compose: true`. |
| `conventions` | Override convention dir names: `shellrc`, `bin`, `conf`. |

### `defaults` — cascading file defaults

```yaml
defaults:
  when: "context=work"   # ANDed into every file's predicate inside this dir
  actions: [source]      # default actions applied to files that don't override
```

### `files` — per-file metadata

A map from true filename (as it exists on disk) to a metadata block. Supports every annotation:

| Field | Equivalent annotation |
|-------|-----------------------|
| `when` | `@when` |
| `after` | `@after` (list) |
| `name` | `@name` |
| `actions` | `@action` (list) |
| `require` | `@require` (list) |
| `request` | `@request` (list) |
| `disable` | `@disable` |
| `link_root` | (no annotation equiv — path config only) |

### Full example

```yaml
when: "os=macos"

link_root: ~/.config/someapp

defaults:
  when: "context=work"
  actions: [source]

files:
  dot-gitconfig-work:
    when: "context=work"
    actions: [link(~/.gitconfig)]
    require: [git]

  dot-gitconfig-personal:
    when: "context=personal"
    actions: [link(~/.gitconfig)]

  settings.json:
    when: "os=macos"
    actions: [link(settings.json)]   # relative to link_root

  legacy-binary:
    disable: true
```

All fields are optional. A `.dagger` with only `defaults.when` is valid. A directory with no `.dagger` is perfectly fine.
