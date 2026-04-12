# dot-dagger Spec Index

**Status:** Draft v1.2 | **CLI:** `dotd` | **Language:** Go

dot-dagger is a dotfiles composition engine — predicate-gated files, DAG-ordered sourcing, symlink management, single generated `init.sh`.

---

## Sections

| File | Contents |
|------|----------|
| [overview.md](overview.md) | §1 Overview, §2 Directory Conventions (`scripts/`, `bin/`, `dots/`, `dot-`, `nosync-`) |
| [dag.md](dag.md) | §3 Logical Names & DAG, §5 Annotations, §6 `.dotd.yaml` |
| [predicates.md](predicates.md) | §4 Predicate System — grammar, env keys, resolution precedence, `exists()` |
| [env.md](env.md) | §7 Config Files (`config.yaml`, `env.yaml`) |
| [shell-init.md](shell-init.md) | §8 Shell Init Integration, §12 Output Style |
| [symlinks.md](symlinks.md) | §9 Symlink Strategy, §10 Drift Detection |
| [cli.md](cli.md) | §11 CLI Interface, §13 Bootstrap |
| [architecture.md](architecture.md) | §14 Project Structure, §15 Dependencies, §16 Design Decisions, §17 Out of Scope, §18 Status |

---

## Quick Reference

- Predicate effective value: `directory_when AND file_when`
- Logical name derivation: strip `nosync-`, strip `dot-`, strip extension — dot-separated from repo root
- Symlink destination: replace `dot-` with `.` at every path level
- Default ordering: alphabetical by logical name within each DAG frontier (Kahn's + alpha tie-break)
- Missing env keys: prompt (TTY) or halt (non-interactive) — never silent

---

[original.md](original.md) — full monolithic spec preserved for reference
