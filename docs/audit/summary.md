# dot-dagger Audit — Executive Summary

Final synthesis of the `dot-dagger` codebase audit. **68 CONFIRMED findings** (`AUDIT-001`–`AUDIT-068`) across seven dimensions. Findings carry verified `file:line` locations and cross-reference their originating agent IDs (A/B/C/D/E/F/T). Two raw findings were DISCARDED (A-004 documented teardown exception; F-007 refuted "stub" lead); seven raw findings were merged into three AUDIT entries.

## Findings per category

| Category | File | Count | AUDIT IDs |
|----------|------|-------|-----------|
| Canonical resolution | `01-canonical-violations.md` | 3 | 001–003 |
| Magic values | `02-magic-values.md` | 9 | 004–012 |
| Duplication | `03-duplication.md` | 9 | 013–021 |
| Ownership | `04-ownership.md` | 4 | 022–025 |
| Design / API quality | `05-design-quality.md` | 7 | 026–032 |
| UX / CLI | `06-ux-cli.md` | 4 | 033–036 |
| Test quality | `07-test-quality.md` | 32 | 037–068 |
| **Total** | | **68** | |

## Severity breakdown

| Severity | Count |
|----------|-------|
| Critical | 3 |
| High | 22 |
| Medium | 26 |
| Low | 17 |
| **Total** | **68** |

## Top Priority Issues

**Critical (all):**
- **AUDIT-001** — `cfg.linkRoot` `$HOME` default re-derived in three consumers; only `apply` is correct, `adopt` hard-errors and `teardown` looks in the wrong directory for the same user config.
- **AUDIT-037** — The entire composition feature ships in production but is end-to-end untested; five compose tests are skipped with false "not yet implemented in v2" reasons and there is no compose e2e.
- **AUDIT-038** — `deriveLinkDest` (empty-dest link derivation, the `dot-`→`.`/`nosync-` home mapping) has 0% coverage — a data-corruption-class path for the user's home dir.

**Most important High:**
- **AUDIT-004** — `packages.yaml` filename has no constant; reader and writer use independent literals, so a one-sided rename silently makes installs no-ops.
- **AUDIT-006** — Annotation/`.dagger` key vocabulary triplicated (`ann*` constants / YAML tags / bare literal); a one-sided rename silently drops `.dagger` DAG edges.
- **AUDIT-013** — `.dagger` `actions:` parsed two divergent ways (dir vs file); the dir parser silently drops unknown action types.
- **AUDIT-022** — `filterWithPrompt` calls `os.Exit(1)` from a prompt helper, usurping `main`'s exit ownership and bypassing cobra's error path (also AUDIT-033 stream/exit-code family).
- **AUDIT-026** — Predicate function registry is wired empty; documented `installed()`/`installable()` predicates are guaranteed runtime errors.
- **AUDIT-027** — The entire `internal/setup` `Scaffold` API is dead — fully built and tested but reached by no shipped command.
- **AUDIT-002** — `config.yaml` path bypasses `ResolvePath`, lacking the flag/env tier every other path has.
- **AUDIT-041** — The brand-new TTY missing-key prompt (the headline of the last 4 commits) has 0% coverage on its interactive path.
- **AUDIT-039 / AUDIT-040** — `internal/fileutil` atomic-write helper and `internal/setup/shell.go` RC-wiring writers have zero direct tests.

## Cross-cutting themes

Several findings converge on the same structural weaknesses. **`.dagger` parsing fragmentation** spans the key vocabulary triplication (AUDIT-006), two divergent action parsers (AUDIT-013), and a third redundant paren-splitter (AUDIT-017) — the highest-risk magic-value/duplication cluster, where a rename silently drops DAG edges. **The compose feature is untested-but-shipping** (AUDIT-037, with AUDIT-038/047 on link derivation, AUDIT-019 on triplicated compose detection, and AUDIT-042 on package glue) — the single largest behavioral gap, compounded by the e2e suite testing a stale release rather than HEAD (AUDIT-062) and a self-skipping Go conflict test (AUDIT-060). **The canonical `cfg.linkRoot` is left empty** by a no-op default fn (AUDIT-001), forcing three downstream re-derivations, mirrored by `config.yaml` bypassing `ResolvePath` (AUDIT-002, AUDIT-003). **Prompt and exit ownership is scattered**: a filter helper owns the process exit (AUDIT-022), three prompt mechanisms disagree on defaults and non-TTY behavior (AUDIT-035), TTY detection is inlined in three ways (AUDIT-018), and the init/setup prompt primitives are bidirectionally coupled (AUDIT-023). **Dead/unwired exported API** recurs (AUDIT-026 predicate registry, AUDIT-027/028 setup scaffold + source-line writers, AUDIT-031 `.Check`/`And`/`DetectInstalled`), inflating surface and misleading readers.

## Provenance

DISCARDED findings (A-004 documented teardown exception, F-007 refuted compose/package "stub" lead) and the full per-finding verdicts (every Phase-2/3 raw finding marked CONFIRMED or DISCARDED with justification) live in `review-log.md`.
