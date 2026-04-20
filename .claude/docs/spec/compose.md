# §21 — Compose Targets

A compose target is a directory whose contents are concatenated into a single generated file. The generated file is then handled exactly like any other file in the same context — sourced if inside `shellrc/`, symlinked if inside `conf/`, placed in the bin dir if inside `bin/`. Compose is a general assembly mechanism, not a config-file-specific feature.

---

## Marking a compose target

A directory is a compose target if and only if its `.dotd.yaml` declares `compose: true`:

```yaml
dotd:
  compose: true
```

There is no implicit detection. The marker is required for clarity and validation.

Compose targets must live inside a convention directory (`shellrc/`, `conf/`, or `bin/`, or their configured equivalents). A compose target outside all convention dirs is a hard error — the output would have no defined behavior.

```
shellrc/dot-aliases.sh.d/    ✓  output sourced in init.sh
conf/dot-tmux.conf.d/        ✓  output symlinked to ~/.tmux.conf
bin/my-tool.d/               ✓  output symlinked to bin dir
tmux/conf/dot-tmux.conf.d/   ✓  topic-grouped, same rules
tools/dot-helper.sh.d/       ✗  error — not inside a convention dir
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

To override, set `dotd.name` in the compose target's `.dotd.yaml` — the directory-level equivalent of `@name` on annotatable files:

```yaml
dotd:
  compose: true
  name: aliases.sh
```

`dotd.name` is a raw name. The `dot-` → `.` transform is not applied to it. It sets the output logical name directly; downstream behavior (sourcing, symlink destination) is then computed from that name using the same rules as any other file in the same convention context.

---

## Output path behavior

Follows the same rules as any other file in the same convention context:

| Context | Behavior |
|---------|----------|
| `shellrc/` | Generated file is sourced into `init.sh`. No symlink. |
| `conf/` | Generated file is symlinked to its destination. The `dot-` → `.` transform applies to the directory basename (after stripping `.d`), relative to `link_root`. |
| `bin/` | Generated file is symlinked to the bin dir and made executable. |

```
shellrc/dot-aliases.sh.d    →  sourced in init.sh
conf/dot-tmux.conf.d        →  strip .d → dot-tmux.conf → .tmux.conf → ~/.tmux.conf
conf/dot-gitconfig.d        →  strip .d → dot-gitconfig → .gitconfig  → ~/.gitconfig
bin/my-tool.d               →  strip .d → my-tool → symlinked to bin dir, executable
```

When `dotd.name` is set, the `dot-` transform is applied to `dotd.name` for `conf/` destinations:

```yaml
dotd:
  compose: true
  name: dot-gitconfig    # conf/ context: symlinked to ~/.gitconfig
```

---

## Generated file location

dotd writes the composed output to:

```
{dot-dagger config dir}/generated/<output-logical-name>
```

Default config dir: `~/.config/dot-dagger`.

```
~/.config/dot-dagger/generated/aliases.sh
~/.config/dot-dagger/generated/tmux.conf
~/.config/dot-dagger/generated/my-tool
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
| `@retain-prefix` | ❌ error |

Invalid annotations on fragments are hard errors at `dotd check` / `dotd apply` time.

---

## Pipeline stage

The compose stage runs between fileset and links:

```
env → fileset → packages → compose → links → init.sh
```

For each compose target with at least one active fragment:
1. Collect active `KindCompose` nodes belonging to this target
2. Order via isolated per-target sub-DAG
3. Concatenate file contents in DAG order
4. Write atomically to the generated file path (temp file + rename, same as `init.sh`)
5. Register a synthetic node of the appropriate kind — `KindScript`, `KindConf`, or `KindBin` — pointing to the generated file, for consumption by the linker and init generator

Compose targets with no active fragments are skipped entirely — no generated file, no synthetic node.

---

## Drift detection

`dotd check` verifies two things per compose target:

1. **Output drift** — the symlink or init.sh entry is correct (same check as any other file of that kind)
2. **Content drift** — the generated file matches what would be produced by the current active fragments; reports stale if not

---

## `dotd compose` subcommands

| Command | Description |
|---------|-------------|
| `dotd compose apply` | Generate all composed files and register synthetic nodes for downstream stages |
| `dotd compose check` | Validate compose targets — report stale or missing generated files |
| `dotd compose list` | List all compose targets and their active fragments |

`dotd apply` and `dotd check` run the compose stage automatically.

---

## Examples

```
shellrc/
  dot-aliases.sh.d/
    .dotd.yaml           ← compose: true
    base.sh              ← always active
    nosync-work.sh       ← @when context=work

conf/
  dot-tmux.conf.d/
    .dotd.yaml           ← compose: true
    base.conf            ← always active
    nosync-work.conf     ← @when context=work
    macos.conf           ← @when os=macos

bin/
  my-tool.d/
    .dotd.yaml           ← compose: true
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

- Compose targets must live inside a convention dir (`shellrc/`, `conf/`, `bin/`). Error if outside.
- Nesting compose targets inside other compose targets is an error.
- The output logical name (derived or via `dotd.name`) must be unique across all compose targets in the repo. Duplicate names are a conflict error.
- Compose targets inside `bin/` produce executable output — the compose stage sets the executable bit on the generated file.
