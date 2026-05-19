# §21 — Compose Targets

A compose target is a directory whose contents are concatenated into a single generated file. What happens to the generated file is determined by the actions declared on the compose target — `source`, `link(dest)`, or both. Compose is a general assembly mechanism; it can live anywhere in the dotfiles repo.

---

## Marking a compose target

A directory is a compose target if and only if its `.dagger` declares `compose: true` (or `actions: [compose, ...]`):

```yaml
compose: true
```

There is no implicit detection. The marker is required for clarity and validation.

A compose target with only `compose: true` assembles the file but does nothing with it — add `source` or `link(dest)` to the `actions:` list to use the output:

```yaml
actions:
  - compose
  - source
```

```yaml
actions:
  - compose
  - link(~/.tmux.conf)
```

---

## Output logical name

The compose target's **output logical name** is derived from its directory basename:

1. Strip leading `nosync-`
2. Strip leading `dot-`
3. Strip trailing `.d` suffix (warn if absent, but do not reject)

```
dot-tmux.conf.d        →  tmux.conf
dot-aliases.sh.d       →  aliases.sh
nosync-dot-work.sh.d   →  work.sh
my-tool.d              →  my-tool   (warn: no .d suffix)
```

To override, set `name:` in the compose target's `.dagger` — the directory-level equivalent of `@name` on annotatable files:

```yaml
name: aliases.sh
actions:
  - compose
  - source
```

`name` is a raw name. The `dot-` → `.` transform is not applied to it. It sets the output logical name directly.

---

## Output behavior

The generated file is handled according to the actions declared on the compose target, in order. The same action types available to any file are valid here — `source`, `link(dest)`, `no-source`:

```yaml
actions:
  - compose
  - source              # source the generated file in init.sh

actions:
  - compose
  - link(~/.tmux.conf)  # symlink the generated file

actions:
  - compose
  - link(~/.tmux.conf)
  - source              # symlink AND source
```

`source` and `link` act on the generated file path, not the compose target directory.

---

## Generated file location

dotd writes the composed output to:

```
{dot-dagger config dir}/generated/<output-logical-name>
```

Default: `~/.local/share/dot-dagger/generated/`.

```
~/.local/share/dot-dagger/generated/aliases.sh
~/.local/share/dot-dagger/generated/tmux.conf
~/.local/share/dot-dagger/generated/my-tool
```

These files are what the linker symlinks or the init generator sources. The user never edits them directly.

---

## Fragments

Files inside a compose target are **fragments** — `KindCompose` nodes. They are content-only: no downstream behavior of their own. The compose target's context determines what happens to the assembled output.

### Fragment ordering

Fragments have an **isolated per-target sub-DAG**. They do not participate in the global shellrc DAG and do not interleave with files from other stages. Within each compose target, ordering is alphabetical by logical name with `@after` for explicit dependencies — same algorithm as shellrc ordering, scoped to the target.

### Active fragments

If a compose target has no active fragments after predicate evaluation, the target is treated as **inactive**: no generated file is written, no symlink is created, no sourcing entry is added. This is consistent with how `@when`-gated files work — a file with a non-matching predicate simply does not participate.

### Fragment logical names

Computed the same way as all other files: relative to the dotfiles repo root, `nosync-`/`dot-` stripped per component, extension stripped from the final component. The `.d` directory component is not stripped from intermediate logical name components.

```
shellrc/dot-aliases.sh.d/base.sh       →  shellrc.aliases.sh.d.base
conf/dot-tmux.conf.d/nosync-work.conf  →  conf.tmux.conf.d.work
```

`@after` references follow the same rules as everywhere else. Path-prefix shorthand is the idiomatic choice within a compose target:

```bash
# @after conf/dot-tmux.conf.d/base.conf
# @after conf/dot-tmux.conf.d/
```

### Valid fragment annotations

| Annotation | Supported |
|-----------|-----------|
| `@when` | ✅ |
| `@after` | ✅ |
| `@name` | ✅ — overrides the fragment's logical name (for `@after` references) |
| `@disable` | ✅ |
| `@symlink` | ❌ error |
| `@source` / `@no-source` | ❌ error |
| `@require` / `@request` | ❌ error |
Invalid annotations on fragments are hard errors at `dotd check` / `dotd apply` time.

---

## Compose as an action

`compose` is one action type in the unified action system (see [actions.md](actions.md)). It assembles a directory's active fragments into a single generated file. Subsequent actions in the same list operate on that generated file.

`compose: true` in `.dagger` is an alias for `actions: [compose]`. Output actions (`link`, `source`) must be declared explicitly — they are never inferred.

To declare output actions, use the `actions:` list directly:

```yaml
actions:
  - compose
  - link(~/.config/tmux/tmux.conf)
```

```yaml
actions:
  - compose
  - source
```

`compose: true` remains valid and fully supported — it is not deprecated.

---

## Pipeline stage

The compose stage runs between fileset and links:

```
env → fileset → packages → compose → links → init.sh
```

For each compose target with at least one active fragment:
1. Collect active `KindCompose` nodes belonging to this target
2. Order via isolated per-target sub-DAG
3. Concatenate fragment contents in DAG order
4. Write atomically to the generated file path (temp file + rename, same as `init.sh`)
5. Execute the remaining declared actions (`source`, `link`) on the generated file path

Compose targets with no active fragments are skipped entirely — no generated file, no synthetic node.

---

## Drift detection

`dotd check` verifies two things per compose target:

1. **Output drift** — the symlink or init.sh entry is correct (same check as any other file of that kind)
2. **Content drift** — the generated file matches what concatenation of the current active fragments would produce; reports stale if not

---

## `dotd compose` subcommands

| Command | Description |
|---------|-------------|
| `dotd compose check` | Validate compose targets — report stale or missing generated files |
| `dotd compose list` | List all compose targets and their active fragments |

`dotd apply` and `dotd check` run the compose stage automatically.

---

## Examples

```
shellrc/
  dot-aliases.sh.d/
    .dagger              ← actions: [compose, source]
    base.sh              ← always active
    nosync-work.sh       ← @when context=work

conf/
  dot-tmux.conf.d/
    .dagger              ← actions: [compose, link(~/.tmux.conf)]
    base.conf            ← always active
    nosync-work.conf     ← @when context=work
    macos.conf           ← @when os=macos

bin/
  my-tool.d/
    .dagger              ← actions: [compose, link(~bin/my-tool)]
    header.sh            ← always active (shebang + common setup)
    nosync-work.sh       ← @when context=work
```

On macOS with `context=work`, all three targets have active fragments:

```
~/.config/dot-dagger/generated/aliases.sh  →  sourced in init.sh
~/.config/dot-dagger/generated/tmux.conf   →  ~/.tmux.conf
~/.config/dot-dagger/generated/my-tool     →  ~/bin/my-tool (executable)
```

On macOS with `context=personal`, `my-tool.d/` has only `header.sh` active. If that is sufficient, it is generated. `conf/dot-tmux.conf.d/` produces `base.conf + macos.conf`.

---

## Constraints

- Compose targets can live anywhere in the dotfiles repo. Output behavior is determined by declared actions, not location.
- Nesting compose targets inside other compose targets is an error.
- The output logical name (derived or via `name` in `.dagger`) must be unique across all compose targets in the repo. Duplicate names are a conflict error.
- `compose` on a file (not a directory) is an error.
- `link` or `source` declared before `compose` in the `actions:` list is an error — the generated file does not exist yet.
