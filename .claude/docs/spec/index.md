# dot-dagger Spec Index

**Status:** Draft v1.3 | **CLI:** `dotd` | **Language:** Go

dot-dagger is a dotfiles composition engine — predicate-gated files, DAG-ordered sourcing, symlink management, single generated `init.sh`.

---

## Sections

| File | Contents |
|------|----------|
| [overview.md](overview.md) | §1 Overview, §2 Directory Conventions (`shellrc/`, `bin/`, `conf/`, `dot-`, `nosync-`) |
| [dag.md](dag.md) | §3 Logical Names & DAG, §5 Annotations, §6 `.dotd.yaml` |
| [predicates.md](predicates.md) | §4 Predicate System — grammar, env keys, resolution precedence, `exists()` |
| [env.md](env.md) | §7 Config Files (`config.yaml`, `env.yaml`) |
| [shell-init.md](shell-init.md) | §8 Shell Init Integration, §12 Output Style |
| [symlinks.md](symlinks.md) | §9 Symlink Strategy, §10 Drift Detection |
| [cli.md](cli.md) | §11 CLI Interface, §13 Bootstrap |
| [architecture.md](architecture.md) | §14 Project Structure, §15 Dependencies, §16 Design Decisions, §17 Out of Scope, §18 Status |
| [package-manifests.md](package-manifests.md) | §20 Package Manifests — `dotd-packages.yaml`, schema, predicate scoping, DAG exclusion |
| [compose.md](compose.md) | §21 Compose Targets — `compose: true`, fragment ordering, generated files, symlink consistency |
| [actions.md](actions.md) | §22 Action System — `@action`, sequencing, convention defaults, aliases |

---

## Quick Reference

- Predicate effective value: `directory_when AND file_when`
- Logical name derivation: strip `nosync-`, strip `dot-`, strip extension — dot-separated from dotfiles repo root
- Symlink destination: replace `dot-` with `.` at every path level (uniform — files and directories follow the same rule)
- `conf/` symlinks to `~` by default; `link_root` in `.dotd.yaml` overrides the base path
- `@symlink` path: absolute if `/` or `~/`, otherwise relative to `link_root`
- Convention dirs recognised until first encounter — `shellrc/conf/` is ignored, `nosync-work/tmux/shellrc/` is valid
- Default ordering: alphabetical by logical name within each DAG frontier (Kahn's + alpha tie-break)
- Missing env keys: prompt (TTY) or halt (non-interactive) — never silent
- Package manifests: `dotd-packages.yaml` / `*.dotd-packages.yaml` — predicate-scoped, no DAG participation
- Compose targets: `compose: true` in `.dotd.yaml` — alias for `actions: [compose]`; fragments → generated file; output action inferred from convention dir or declared explicitly
- Actions: `@action <type>` — `compose`, `link(dest)`, `source`, `no-source`; multiple actions applied in order; `@source`/`@no-source`/`@symlink` are aliases
- Compose pipeline: env → fileset → packages → **compose** → links → init.sh

---
