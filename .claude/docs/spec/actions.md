# §22 — Action System

An **action** declares what to do with a file or directory. Multiple actions can be declared on a single node and are applied in declaration order.

Actions are declared via `@action` annotations on files, or via the `actions:` key in `.dotd.yaml` for directories. The existing specific annotations (`@source`, `@no-source`, `@symlink`) and `.dotd.yaml` keys (`compose: true`) are aliases for specific action types and remain valid.

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

## `.dotd.yaml` syntax

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

## Convention dir defaults

Convention directories carry implicit default actions that apply when no explicit action of that type overrides them:

| Directory | Implicit action |
|-----------|----------------|
| `shellrc/` | `source` |
| `conf/` | `link(<dot-transform of filename relative to link_root>)` |
| `bin/` | `link(<bin-dir>/<name>)` |

An explicit `@action` of the same type as a convention default replaces the convention default for that file. An explicit `@action` of a different type adds to it.

Example: a file in `conf/` with `@action link(~/custom/dest)` links to `~/custom/dest` instead of the convention-derived destination. A file in `shellrc/` with `@action link(~/dest)` both sources (convention) and links (explicit).

Convention dirs are a convenience layer over this system. The goal is that explicit actions can always replace or extend convention behaviour, and convention dirs can be eliminated or reconfigured without changing the action model.

---

## Aliases

These existing annotations and keys remain valid and are treated as aliases:

| Alias | Equivalent |
|-------|-----------|
| `@source` | `@action source` |
| `@no-source` | `@action no-source` |
| `@symlink <dest>` | `@action link(<dest>)` |
| `compose: true` in `.dotd.yaml` | `actions: [compose]` |

`@require` and `@request` are **not** actions — they are package dependency declarations and remain their own annotation type.

---

## Errors

- `compose` on a file: hard error — `@action compose` is only valid on directories
- `link` without a destination argument: hard error
- `link` or `source` declared before `compose` on a compose target: hard error — the result does not exist yet
- Duplicate `link(dest)` with conflicting destinations on the same node: hard error
