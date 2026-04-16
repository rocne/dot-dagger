# §3, §5, §6 — Logical Names, Annotations & .dot-dagger.yaml

## 3. Logical Names and the DAG

Every file is a node in the DAG, identified by its logical name — a dot-separated path from the dotfiles repo root with the file extension stripped.

### Name derivation

Applied per path component, in order:
1. Strip leading `nosync-`
2. Strip leading `dot-` (after nosync-)
3. Strip file extension from the final component only

```
tmux/scripts/helpers.sh         → tmux.scripts.helpers
scripts/math.sh                 → scripts.math
nosync-work/scripts/aliases.sh  → work.scripts.aliases
conf/dot-config/tmux/tmux.conf  → conf.config.tmux.tmux
nosync-dot-secrets/api.sh       → secrets.api
```

The logical name always reflects the full path. No segments are skipped regardless of whether intermediate directories have `.dot-dagger.yaml` files.

### `@name` — aliasing

`@name` replaces the entire logical name, not just the last segment. It creates a canonical identity for the file independent of its filesystem location:

```bash
# tmux/scripts/tmux-helpers-macos.sh
# @name tmux.scripts.helpers
```

This is the primary tool for variant files — two files that represent the same logical unit under different conditions both declare the same `@name`. The resolver enforces that exactly one is active at a time. Predicates must be mutually exclusive.

### `@after` references

`@after` accepts either a full logical name or a path prefix ending in `/`:

```bash
# @after tmux.scripts.helpers   — specific file by logical name
# @after tmux/                  — all active files under tmux/
# @after tmux/scripts/          — all active files under tmux/scripts/
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

1. If the first line is a shebang (`#!`), skip it and allow one immediately-following blank line
2. Read contiguous lines starting with `#` (all leading `#` stripped) or `//`
3. Non-`@` comment lines are ignored without stopping the scan
4. Any blank line or non-comment line stops the scan

### Supported annotations

| Annotation | Purpose |
|-----------|---------|
| `@when <expr>` | Inclusion predicate. Multiple lines are ANDed. |
| `@after <ref>` | DAG ordering dependency — logical name or path prefix ending in `/` |
| `@name <logical-name>` | Override the full logical name of this file |
| `@symlink <path>` | Symlink this file to an explicit destination (see symlinks.md for path rules) |
| `@retain-prefix` | Opt out of `dot-` transformation for this path component |
| custom | Dispatched to registered external handlers |

### `@symlink`

Opts any file into symlinking at an explicit destination. Rarely needed — `conf/` handles the common case. Used for files outside `conf/` that need symlinking, or for config files that need a non-conventional destination.

Files in `conf/` are symlinked by convention — `@symlink` is only needed there to override the default destination. `@symlink` takes precedence over convention in all cases. See [symlinks.md](symlinks.md) for destination path rules.

### Custom annotations

Unknown annotations are not errors. They are dispatched to registered external handlers declared in `config.yaml`:

```yaml
annotation_handlers:
  requires: dag-pkg procure
```

So `@requires fzf` becomes `dag-pkg procure fzf` at apply time.

In `--dry-run` mode, annotation handlers are not invoked. The tool prints what would be called instead.

If a handler is not installed or fails, the tool warns but continues by default. With `--no-interactive`, failures halt execution.

---

## 6. `.dot-dagger.yaml`

An optional metadata file that can appear in any directory. Its primary purpose is to provide metadata for files that cannot carry annotations — JSON, binary, XML, and other formats without supported comment syntax.

`.dot-dagger.yaml` has three sections:

**`directory`** — properties of the directory node itself. Does not cascade to contents. The `when` field gates traversal of the entire subtree; if false, the directory is not entered at all.

**`defaults`** — values that cascade to all files inside the directory and all subdirectories, ANDed with each file's own annotations.

**`files`** — per-file metadata for specific files that cannot carry annotations.

```yaml
# Properties of this directory node itself
directory:
  when: "os=macos"        # gates traversal — don't enter unless this matches
  retain_prefix: true     # don't transform dot- on this directory's name in symlink paths
  link_root: ~/.config/someapp  # override symlink destination root for this subtree

# Defaults that cascade to all files inside
defaults:
  when: "context=work"

# Per-file metadata for files that cannot carry annotations
# path: true filename as it exists on disk
# when/after/name/symlink: use logical name semantics, same as annotations
files:
  - path: dot-gitconfig-work
    when: "context=work"
    symlink: ~/.gitconfig

  - path: dot-gitconfig-personal
    when: "context=personal"
    symlink: ~/.gitconfig

  - path: settings.json
    symlink: "settings.json"   # relative to link_root
    when: "os=macos"
    retain_prefix: true
```

All sections are optional. A `.dot-dagger.yaml` with just a `defaults.when` is valid. A directory with no `.dot-dagger.yaml` is also perfectly fine.

Declaring a predicate in `.dot-dagger.yaml` for a specific file AND as an annotation in that file is an error. All predicate expressions are validated at load time.
