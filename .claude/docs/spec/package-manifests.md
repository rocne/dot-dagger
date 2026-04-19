# §20 — Package Manifests

Package manifests are YAML files that declare packages to install, optionally scoped by predicate. They are the intended mechanism for listing personal or system-level packages that are not tied to any specific shellrc script.

---

## File naming

A file is treated as a package manifest if its name matches:

```
dotd-packages.yaml
*.dotd-packages.yaml
```

The `dotd-packages.yaml` prefix is intentional — it avoids collisions with any `*.packages.yaml` files a user may already have for unrelated tools or systems.

Package manifests may appear anywhere in the dotfiles tree. They are not tied to any convention directory (`shellrc/`, `bin/`, `conf/`). The `nosync-` prefix applies normally — a `nosync-work.dotd-packages.yaml` is stripped and gitignored.

---

## Schema

A manifest is a YAML sequence of blocks. Each block declares an optional predicate and a list of packages:

```yaml
- packages:
    - ripgrep
    - fd
    - bat

- when: os=macos
  packages:
    - aerospace
    - rectangle

- when: os=linux
  packages:
    - i3

- when: os=macos AND context=work
  packages:
    - some-work-tool
```

| Field | Required | Description |
|-------|----------|-------------|
| `when` | No | Predicate string. Same grammar as `@when` in shellrc files. Omit for unconditional. |
| `packages` | Yes | List of package names. |

---

## Predicate evaluation

Effective predicate for a block follows the same rule as all other files:

```
directory_when AND block_when
```

- **`directory_when`**: the `when` declared in the nearest ancestor `.dotd.yaml`, if any. If the directory predicate is false, the entire manifest file is skipped.
- **`block_when`**: the `when` declared on the block itself. Omitted = always true.

To apply a predicate to an entire manifest, place the file in a directory with a `.dotd.yaml` `when` declaration. No file-level `when` key is needed or supported.

---

## Integration with `dotd package` commands

Package manifests contribute to the same package catalog as `@request` annotations in shellrc files. All `dotd package` subcommands include packages from both sources:

| Command | Behaviour |
|---------|-----------|
| `dotd package list` | Lists all packages from active blocks across all manifests and shellrc annotations |
| `dotd package check` | Reports install status of all active packages |
| `dotd package generate` | Generates install script covering all active packages |

---

## DAG

Package manifests do not participate in the DAG. They are not sourced into `init.sh` and have no logical name, `@after` ordering, or `@name` aliasing. They are collected independently during the walk and fed directly to the package catalog.

---

## Multiple manifests

Any number of manifest files may exist in the repo. All active manifests are collected and their packages unioned. Typical layouts:

```
dotd-packages.yaml              # unconditional personal tools
mac.dotd-packages.yaml          # mac-specific, scoped by block when
work/dotd-packages.yaml         # in a dir with when: context=work
nosync-local.dotd-packages.yaml # gitignored, machine-local overrides
```
