# Design: `dotd adopt` — v2 Implementation

**Date:** 2026-05-17
**Status:** Approved for implementation

---

## Overview

`dotd adopt <file>` moves a file into the dotfiles repo and replaces the original with a symlink. It is the inverse of `dotd apply` for a single file — bringing an unmanaged file under dot-dagger management in one command.

The command was stubbed in v1→v2 migration. This design implements it against the v2 pipeline.

---

## Behaviour

```
dotd adopt ~/.bashrc
  → infer destination: conf/dot-bashrc  (dotfile)
  → prompt: "conf/dot-bashrc — adopt and replace with symlink? [Y/n]"
  → copy ~/.bashrc → <dotfiles>/conf/dot-bashrc
  → remove ~/.bashrc
  → create symlink ~/.bashrc → <dotfiles>/conf/dot-bashrc
```

For `shellrc/` files, no symlink is created (files are sourced, not linked). Adopt prints:
> added to shellrc/ — run `dotd apply` to regenerate init.sh

### Flags

Adopt-specific flags:

| Flag | Description |
|------|-------------|
| `--to <rel>` | Override inferred destination (relative to dotfiles root) |
| `--yes` / `-y` | Skip confirmation prompt |

Global flags honoured by adopt (declared on root command):

| Flag | Behaviour in adopt |
|------|-------------------|
| `--dry-run` | Print plan without copying, removing, or symlinking |
| `--force` | Passed through to `Act` — overwrites non-owned symlinks at destination |

### Inference rules

Applied in priority order:

| Condition | Destination |
|-----------|-------------|
| Executable bit set | `<bin>/<name>` |
| Extension `.sh` `.bash` `.zsh` `.fish` | `<shellrc>/<name>` |
| Hidden file (`.bashrc`, `.gitconfig`, …) | `<conf>/dot-<name>` |
| Extension `.conf` `.config` `.toml` `.yaml` `.yml` `.ini` `.cfg` `.json` | `<conf>/<name>` |
| None of the above | error — use `--to` |

Where `<shellrc>`, `<bin>`, `<conf>` are the configured convention dir names (default: `shellrc`, `bin`, `conf`).

Convention names are loaded from the root `.dagger` file (`conventions:` key). If absent, defaults apply.

### `--to` behaviour

If `--to` ends with `/`, the source filename is appended. Otherwise the value is used as-is. Path is relative to dotfiles root.

---

## Architecture

### New: `internal/dagger` — `Conventions` field

Add to `ComposableNode`:

```go
Conventions struct {
    Shellrc string `yaml:"shellrc"`
    Bin     string `yaml:"bin"`
    Conf    string `yaml:"conf"`
} `yaml:"conventions"`
```

Defaults (`shellrc`, `bin`, `conf`) are applied in adopter when fields are empty.

### New: `internal/adopter/adopter.go`

```go
package adopter

type ConventionNames struct {
    Shellrc string
    Bin     string
    Conf    string
}

func DefaultConventions() ConventionNames {
    return ConventionNames{Shellrc: "shellrc", Bin: "bin", Conf: "conf"}
}

type Inference struct {
    DestRel string // e.g. "conf/dot-bashrc"
    Reason  string // e.g. "dotfile (dot- prefix added)"
    Unknown bool
}

type AdoptOptions struct {
    DotfilesRoot string
    LinkRoot     string
    BinDir       string
    Force        bool
    DryRun       bool
}

// Infer returns the inferred dotfiles-relative destination for src.
func Infer(src string, info os.FileInfo, conv ConventionNames) Inference

// Adopt copies src to <dotfilesRoot>/<destRel>, removes src, and creates
// a symlink at the original src path. Returns the ActResult from pipeline.Act.
func Adopt(src, destRel string, opts AdoptOptions) (*pipeline.ActResult, error)
```

### Updated: `cmd/dotd/adopt.go`

Thin CLI layer:
1. Load root `.dagger` → read `Conventions` (fill defaults for empty fields)
2. `os.Stat(src)` — validate exists, not a dir
3. Resolve `destRel` from `--to` flag or `Infer`
4. If inference fails and no `--to`: error
5. If not `--yes` and stdin is a TTY: prompt confirmation (huh). If not a TTY, behave as `--yes`.
6. Call `adopter.Adopt`
7. Log result

---

## Data flow

```
adopt.go
  │
  ├─ dagger.LoadFile(<dotfiles>/.dagger) → ConventionNames
  ├─ os.Stat(src)
  ├─ adopter.Infer(src, info, conv) → Inference
  │
  ├─ [prompt if !yes && tty]
  │
  └─ adopter.Adopt(src, destRel, opts)
       ├─ copyFile(src, destAbs)           // mkdir -p + copy
       ├─ os.Remove(src)
       ├─ build RawNode{
       │    Path:        destAbs,
       │    LogicalName: node.DeriveName(rel),
       │    Actions:     [{Type: "link"}],          // conf/ and bin/
       │    LinkRoot:    opts.LinkRoot,
       │    LinkRootDir: filepath.Dir(destAbs),     // the conf/ dir
       │  }
       └─ pipeline.Act([node], ActOptions{...})
            └─ createSymlink: symlink at src_original → destAbs
```

For `bin/` files, `Actions` is `[{Type: "link", Dest: filepath.Join(binDir, name)}]` — explicit dest, no `deriveLinkDest` needed.

For `shellrc/` files, `Actions` is `[{Type: "source"}]` — `Act` returns node in `Sourced`, no symlink created.

---

## Error handling

| Case | Behaviour |
|------|-----------|
| `src` not found | error before any writes |
| `src` is a directory | error — one file at a time |
| `destAbs` already exists | error — will not overwrite |
| Inference fails, no `--to` | error with hint to use `--to` |
| Copy succeeds, remove fails | error — dest file present, original intact, no symlink (safe state) |
| Remove succeeds, `Act` fails | error — prints recovery hint: `mv <destAbs> <src>` |
| Dry-run | print plan only — no copy, no remove, no symlink |

---

## Dependencies

- `github.com/charmbracelet/huh` — interactive prompt (was removed when adopt was stubbed; must be re-added via `go get`)
- `github.com/charmbracelet/x/term` — TTY detection (already an indirect dep; adopt.go uses it directly, so it becomes a direct dep)

---

## Testing

`internal/adopter` is unit-tested independently:

| Test | Coverage |
|------|----------|
| `TestInfer_*` | Each inference rule: executable, shell ext, hidden, config ext, unknown |
| `TestResolveToFlag` | `--to` with trailing slash, without trailing slash (CLI layer, not adopter) |
| `TestAdopt_Conf` | conf/ file: copy, remove, symlink created |
| `TestAdopt_Bin` | bin/ file: copy, remove, symlink to binDir |
| `TestAdopt_Shellrc` | shellrc/ file: copy, remove, no symlink, node in Sourced |
| `TestAdopt_DestExists` | error when dest already in dotfiles |
| `TestAdopt_DryRun` | no filesystem changes |
| `TestAdopt_Force` | --force passed through to Act |

CLI layer (`adopt.go`) tested via existing integration test harness if applicable; otherwise manual.

---

## Out of scope

- Adopting directories (one file at a time)
- Annotating the adopted file (convention dir provides implicit action — no annotation needed)
- Running full pipeline after adopt (user runs `dotd apply`)
- Migrating to approach C (Walk-based `RawNode` derivation) — possible later without API change
