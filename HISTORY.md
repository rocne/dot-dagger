# Project History

A condensed record of major development phases. The working documents that
drove each phase (plans, specs, audits, session handoffs) were removed from
the working tree on 2026-06-12 — they are preserved in git history. The last
commit containing all of them is `691e8d4`.

To recover any removed document:

```sh
git show 691e8d4:docs/audit/summary.md            # view one file
git ls-tree -r --name-only 691e8d4 -- docs/audit docs/superpowers   # list all
```

---

## Timeline

| When | Phase | Outcome |
|------|-------|---------|
| 2026-05-07 | **v2 rewrite** — foundation, env/config, pipeline, CLI/init plans | New annotation scanner (`@key(args)` syntax), `.dagger` per-directory config (replacing `.dotd.yaml`), node type hierarchy with logical-name derivation, five-stage pipeline (walk → filter → order → act → init.sh). |
| 2026-05-14 | **Unified action system** | Canonical `@action <type>` annotation; `@source`/`@no-source`/`@link`/`@symlink` normalized through one conversion point; `ValidateNodes` sequencing stage between walk and filter. |
| 2026-05-15 | **CLI completeness** (PR #62) | `dotd env diff`, `dotd package list`; shared pipeline runner; stable subcommands unhidden; spec synced to implementation. |
| 2026-05-17 | **`dotd adopt`** (PR #63, v0.2.22) | New `internal/adopter` package; moves a file into the repo and replaces the original with a symlink; destination inference rules. |
| 2026-05-18 | **Spec ↔ code audit** (PR #64, v0.2.23) | Full cross-reference of spec against implementation; all high/medium items fixed. |
| 2026-05-23 | **Lifecycle commands + command groups** | Monolithic `init` split into `setup` (system config) / `init` (scaffold) / `apply`–`unapply` (reconcile) / `teardown` (remove config). `--help` organized into labeled groups. |
| 2026-05-25–28 | **E2E Docker testing**; repo went public | Ubuntu e2e suite (binary, installer, combined; PRs #77–78, v0.2.34); failures auto-open a GitHub issue; `install.sh` curl-only (PR #76). |
| 2026-05-31–06-03 | **Full codebase audit + remediation** (PRs #89, #95, et al.) | 7-dimension audit (canonical sources, magic values, duplication, ownership, design, CLI UX, test quality): 75 raw → 68 confirmed findings (`AUDIT-001`–`068`; 3 Critical, 22 High, 26 Medium, 17 Low). All 68 remediated across nine workstreams — see `AUDIT-NNN` commit messages. |
| 2026-06-04–07 | **E2E coverage expansion** | Shell e2e + Go integration tests for every command across five batches; `runWithStdin` test harness; prompt I/O consolidation. |
| 2026-06-08–11 | **CLI UX audit** (PRs #97–#105) | 17 findings, all closed: `error:`/`hint:` convention, exit-code dichotomy (1 runtime / 2 usage), Long+Examples on every command, `--json` on all data commands, `setup --non-interactive`, scoped flag visibility, uniform `env show` format. |

## Removed document index

What lived where (all recoverable via the commands above):

- `docs/audit/` — the 2026-05/06 codebase audit: codebase map, per-dimension
  findings (`01`–`07`), independent review log (73 confirmed / 2 discarded),
  executive summary.
- `docs/superpowers/plans/` — implementation plans for each phase above.
- `docs/superpowers/specs/` — design specs (v2 redesign, e2e coverage).
- `docs/superpowers/handoffs/` + `audits/` — session handoffs and the
  2026-05-18 spec/code audit.
