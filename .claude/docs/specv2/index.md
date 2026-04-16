# dotr Spec v2 — Index

**Status:** Draft v0.1 | **Suite command:** `dotr` | **Language:** Go

dotr is a suite of composable shell management tools. Each tool is independently useful and importable as a Go library. The top-level `dotr` orchestrator combines them into a full shell environment management system.

---

## What changed from v1

v1 specified a single `dotd` binary. v2 redesigns this as a suite of focused tools sharing a common annotation system.

| v1 | v2 |
|----|-----|
| Single `dotd` binary | Suite: `dota`, `dotd`, `dotl`, `dotp`, `dotr` |
| Single repo (`dot-dagger`) | Multiple repos, one per tool |
| Annotation dispatch as extension point | `dota` as shared annotation + predicate bus |
| Package management noted as separate | `dotp` fully specified as suite member |

---

## Sections

| File | Contents |
|------|----------|
| [suite.md](suite.md) | Tool suite overview, ownership, composition model, `.dot-dagger.yaml` |
| [structure.md](structure.md) | Repo/module layout, internal packages, FileSet, I/O boundary, standalone vs orchestrated |
| [annotation.md](annotation.md) | `dota` — annotation system, predicate extension, custom handlers, unknown annotation behavior |
| [dote.md](dote.md) | `dote` — environment resolution, `env.yaml`, custom detectors, CLI |
| [dotp.md](dotp.md) | `dotp` — package management, `@require`/`@request`, registry, priority |

> Remaining v1 sections (predicates, DAG, symlinks, shell-init, CLI) carry forward with adjustments noted in [suite.md](suite.md). Full rewrites deferred until design stabilises.

---

## Quick Reference

- **`dota`** — annotation parsing, predicate engine, extension bus (library only, no env dependency)
- **`dote`** — env resolution, `env.yaml`, custom detectors; `dote show` for debugging
- **`dotd`** — file selection, DAG resolution, `init.sh` generation; owns `scripts/`
- **`dotl`** — symlink apply/remove/check; owns `conf/`, `bin/`
- **`dotp`** — package management; `@require` (hard gate) and `@request` (soft ask); `installed()`/`installable()` predicates
- **`dotr`** — orchestrator; wires tools together into full shell management
- **`.dot-dagger.yaml`** — per-directory config for non-annotatable files; sectioned by tool
