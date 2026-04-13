# Suite Architecture

## Tools

| Tool | CLI | Type | Uses | Owns | Repo |
|------|-----|------|------|------|------|
| `dota` | none | library only | — | Annotation parsing, predicate engine | `dota` |
| `dote` | `dote` | binary + library | `dota` | `env.yaml`, environment detection | `dote` |
| `dotd` | `dotd` | binary + library | `dota`, `dote` | `scripts/`, DAG, `init.sh` | `dotd` |
| `dotl` | `dotl` | binary + library | `dota`, `dote` | `conf/`, `bin/` | `dotl` |
| `dotp` | `dotp` | binary + library | `dota`, `dote` | Package management | `dotp` |
| `dotr` | `dotr` | binary + library | all | Orchestrator | `dotr` |

**Design goal: composability parity.** The behaviour achievable through `dotr` must be straightforwardly reproducible by composing the individual tools. A user who wants only part of the system — or who wants to script their own orchestration — should be able to reach the same outcome without `dotr`. `dotr` is a convenience, not a gate.

---

## Dependency Graph

```
dota  ←──────────────────────────────────────────┐
  ↑                                               │
dote (uses dota)                                  │
  ↑         ↑          ↑          ↑              │
dotd       dotl       dotp       ...             │
  └─────────┴──────────┴──────────┴──── dotr ────┘
```

`dota` has no dependencies within the suite. `dote` depends only on `dota`. All other tools depend on both.

---

## Directory Ownership

Special directories in a dotfiles repo are owned by specific tools:

| Directory | Owned by | Purpose |
|-----------|----------|---------|
| `scripts/` | `dotd` | Shell scripts to source; DAG-ordered into `init.sh` |
| `conf/` | `dotl` | Config files symlinked to `~` (or `link_root`) |
| `bin/` | `dotl` | Executables symlinked onto PATH |

---

## Standalone vs Orchestrated Mode

**Standalone:** Each tool operates on its owned directories independently. No predicate filtering — `dotl` standalone walks `conf/` and `bin/` and links everything. `dote` provides env context.

**Orchestrated (`dotr`):** `dotd` runs file selection with full predicate evaluation (including registered extensions from `dotp`). `dotr` passes the resulting filtered file list to `dotl` and `dotp`, which act on what they receive.

---

## Composition Model

In orchestrated mode (`dotr`):

1. `dote` resolves the environment from `env.yaml` and system detectors
2. `dotp` registers its annotation handler and `installable()` predicate with `dota`
3. `dotd` runs file selection — evaluates predicates against the dotfiles tree using env from `dote`; produces the active file set
4. `dotp` acts on active files — installs packages declared via `@package`
5. `dotl` acts on active files — applies symlinks for `conf/` and `bin/`
6. `dotd` generates `init.sh` from the active `scripts/` set

---

## Config File

Per-directory metadata for non-annotatable files is stored in `.dotr.yaml`. All tools in the suite read the section relevant to them.

```yaml
dote:
  # environment overrides for this subtree

dotd:
  when: "os == macos"
  defaults:
    when: "os == macos"

dotl:
  link_root: ~/.config/nvim
```

Named `.dotr.yaml` to acknowledge the suite ecosystem. Each tool reads only its own section.

---

## What v1 sections carry forward

Most v1 design decisions apply unchanged, attributed to the tool that now owns them:

| v1 section | Now owned by |
|-----------|-------------|
| Predicate grammar and evaluation | `dota` |
| Environment detection, `env.yaml` | `dote` |
| DAG, logical names, annotations | `dotd` (uses `dota`, `dote`) |
| Symlink strategy, drift detection | `dotl` (uses `dota`, `dote`) |
| Shell init generation | `dotd` |
| `.dotd.yaml` config files | renamed to `.dotr.yaml` |

### Open questions

- `link_root` and `@symlink` relative path semantics — still needs validation against real use cases (originally deferred in v1)

---

## Repository

Single repository (`dotr`), single Go module. All tools live in `cmd/`; all logic in `internal/`. See [structure.md](structure.md) for the full layout.

The current `dot-dagger` repo will be retired when `dotr` is created.
