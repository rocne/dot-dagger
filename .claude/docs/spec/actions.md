# §22 — Action System

An **action** declares what to do with a file or directory. Multiple actions can be declared on a single node and are applied in declaration order.

Actions are declared via `@action` annotations on files, or via the `actions:` key in `.dagger` for directories. The existing specific annotations (`@source`, `@no-source`, `@symlink`) and `.dagger` keys (`compose: true`) are aliases for specific action types and remain valid.

---

## Action types

| Action | Applies to | Description |
|--------|-----------|-------------|
| `compose` | directories only | Assemble active fragments (DAG-ordered) into a single generated file |
| `link(dest)` | files, directories | Symlink this file (or composed result) to `dest` |
| `source` | files, directories | Include in `init.sh` sourcing |
| `no-source` | files, directories | Exclude from `init.sh` sourcing |

`compose` on a file is a hard error with a clear message. All other action types applied to a directory operate on the composed result — they require `compose` to appear earlier in the action list.

---

## Annotation syntax

```
@action compose
@action source
@action no-source
@action link(~/dest)
@action link(relative/path)
```

Multiple `@action` lines are allowed and applied in declaration order:

```bash
#!/bin/sh
# @action compose
# @action link(~/.tmux.conf)
```

---

## `.dagger` syntax

For directories, `actions:` is a list applied in order:

```yaml
dotd:
  actions:
    - compose
    - link(~/.tmux.conf)
```

Single-action shorthand (string, not list) is valid when only one action is needed:

```yaml
dotd:
  actions: source
```

---

## Sequencing

Actions are applied left-to-right (top-to-bottom in annotations). The primary constraint is that `compose` must precede any action that operates on the result (`link`, `source`).

Valid sequences:
```
compose → link(dest)     # assemble then symlink
compose → source         # assemble then source in init.sh
compose → link → source  # assemble, symlink, and source
source                   # source directly (no compose)
link(dest)               # symlink directly (no compose)
```

`link` and `source` are independent output actions — both can appear on the same node. Their relative order does not affect behaviour.

---

## Convention dirs

Convention directories (`shellrc/`, `bin/`, `conf/`) are not special at the action-system level. Their default actions come from `defaults.actions` in the directory's `.dagger` file — the same mechanism as any other directory. The convention is naming + prepopulated defaults, not implicit magic.

---

## Aliases

These existing annotations and keys remain valid and are treated as aliases:

| Alias | Equivalent |
|-------|-----------|
| `@source` | `@action source` |
| `@no-source` | `@action no-source` |
| `@symlink(dest)` | `@action link(dest)` |
| `compose: true` in `.dagger` | `actions: [compose]` |

`@require` and `@request` are **not** actions — they are package dependency declarations and remain their own annotation type.

---

## Errors

- `compose` on a file: hard error — `@action compose` is only valid on directories
- `link` without a destination argument: hard error
- `link` or `source` declared before `compose` on a compose target: hard error — the result does not exist yet
- Duplicate `link(dest)` with conflicting destinations on the same node: hard error
