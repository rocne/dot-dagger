# Annotations

Annotations are metadata written as comments at the top of a file. They tell dotd when a file should be active, what it depends on, which packages it needs, and how it should be processed.

```sh
#!/bin/bash
# @when os=macos AND context=work
# @after scripts/base/
# @require ripgrep

# --- script body below ---
alias ls='ls -G'
```

Annotations are **read at apply time, not at shell startup**. They have zero effect at runtime — your shell sources a pre-built `init.sh` that contains only the files that were active when `dotd apply` last ran.

## How scanning works

Scanning begins at the first line of the file. If the first line is a **shebang** — a line starting with `#!` that tells the OS which interpreter to run the file with (e.g. `#!/bin/bash`) — it is skipped. Scanning then reads comment lines and stops at the first blank line or non-comment line.

| File type | Comment prefix |
|---|---|
| Shell (`.sh`, `.bash`, `.zsh`, `.fish`) | `#` |
| C-style (`.c`, `.go`, `.js`, `.ts`, etc.) | `//` |
| Any file with `#` or `//` comments | Works |
| JSON, binary, XML, etc. | Use [`.dotd.yaml`](../reference/dotd-yaml.md) |

## What annotations exist

| Annotation | Purpose |
|---|---|
| [`@when`](../reference/annotations.md#when) | Gate file inclusion on a condition |
| [`@name`](../reference/annotations.md#name) | Override the file's logical name |
| [`@after`](../reference/annotations.md#after) | Declare a load-order dependency |
| [`@symlink`](../reference/annotations.md#symlink) | Symlink to an explicit path |
| [`@retain-prefix`](../reference/annotations.md#retain-prefix) | Keep `dot-`/`nosync-` prefix on the filename |
| [`@require`](../reference/annotations.md#require) | Hard package gate |
| [`@request`](../reference/annotations.md#request) | Soft package ask |
| [`@disable`](../reference/annotations.md#disable) | Exclude from all processing |
| [`@no-source`](../reference/annotations.md#no-source) | In load order, not sourced |
| [`@source`](../reference/annotations.md#source) | Force-source regardless of directory |

## Files without comment syntax

JSON, compiled binaries, XML, and other files that don't support comments can't carry annotations directly. Use `.dotd.yaml` in the same directory to provide equivalent metadata:

```yaml
dotd:
  files:
    - path: settings.json
      when: "os=macos"
      disable: false
```

See [`.dotd.yaml` reference](../reference/dotd-yaml.md) for the full format.

## Convention vs annotation

Most files don't need any annotations at all. The conventions handle the common cases:

- Files in `scripts/` are automatically sourced in `init.sh`
- Files in `conf/` are automatically symlinked into `$HOME`
- Files in `bin/` are automatically symlinked onto `$PATH`

Annotations are for exceptions: "source this only on macOS", "source this after that", "this needs ripgrep installed first".
