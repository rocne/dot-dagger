# Audit Trail — 2026-06-12

Full-project audit: inconsistencies, UX problems, doc accuracy. Companion docs:
- `audit-2026-06-12-findings.md` — findings with category + priority
- `audit-2026-06-12-plan.md` — execution plan (written for a lower-performing model)

Prior audit `.claude/docs/cli-ux-audit-2026-06-08.md` had all 17 findings closed
(PRs #97–#105). This audit covers fresh ground: docs accuracy, fresh-machine UX,
scriptability, repo hygiene.

---

## Method

1. Read prior audit + audit-guide + TODO + latest handoff (avoid re-finding closed items).
2. Ran audit-guide grep passes 1 and 3 (magic values, canonical sources, output routing).
3. Built `dotd` (`go build`), ran `go vet`, full `go test ./...` — all clean.
4. Live-exercised CLI in sandbox `HOME=/tmp/audit-home`:
   - fresh machine (no config): `apply`, `check`, `list`, `env show`, `config show`
   - error paths: unknown command, typo suggestion, unknown key, missing arg, unknown flag
   - scripted bootstrap: `setup -n`, `init` (non-TTY, `yes |`, correct piped answers)
   - happy path: setup → init → add files → apply → check
   - exit-code verification (without pipes — `head` pollutes `$?`)
5. Cross-checked README.md + docs/ (mkdocs site) against actual `--help` output.
6. Checked tracked files vs CLAUDE.md commit policy; mkdocs nav vs tracked docs.

## Key evidence

**Fresh machine, no config, `dotd apply` from /tmp:**
```
WARN nosync- path not gitignored: /tmp/TestAnnotate_NonTTY.../nosync-work.conf ...
error: walk /tmp: dagger: open /tmp/systemd-private-...: open /tmp/systemd-private-...: permission denied
EXIT:1
```
→ cwd fallback walked all of /tmp; foreign warnings; doubled path in error; no hint.

**Silent no-op:** files in `shellrc/`+`config/` but no `.dagger` (init skipped in
non-TTY): `list` shows 2 nodes, `check` says "2 active / 2 total", but
`apply` → "0 symlinks applied", "(0 nodes)", exit 0, no warning.

**`yes | dotd init`:** created directory literally named `y` — name prompt
consumed the next "y" line; all three convention sections wrote
`dotfiles/y/.dagger`. Non-TTY without piped input (EOF) silently skips all
prompts instead. `init` has no `-y`/`--non-interactive`; `setup` has `-n`.

**`setup -n`:** prints section labels + "Accepting shown defaults." but never
shows the values written to config.yaml.

**Doc/CLI divergence (verified against `--help`):**
- No `dotd files`, no `dotd link`, no `dotd dag apply` (dag has only `check`)
- No `--verbose` flag anywhere (`error: unknown flag: --verbose`, exit 2)
- `setup` flag is `-n/--non-interactive`, not `--yes`
- `adopt` *moves* + replaces with symlink ("Move a file into the dotfiles repo
  and replace it with a symlink"); has `--to`, `-y/--yes` only — no `--no-interactive`
- `env set` is positional `<key> <value>`, not `key=value`
- `--link-root` default is `$HOME`, not `~/.config`
- Source-line wiring lives in `init` (`maybeAddSourceLine`, init_cmd.go:64), not `setup`

**Exit codes verified correct:** unknown flag → 2, missing arg → 2, runtime → 1,
data commands → 0. (An earlier "exit 0 on unknown flag" reading was a `| head`
pipe artifact.)

**Greps clean:** no output-routing violations; no canonical-path violations
(teardown's `ecosystem.DefaultConfigFile()` call is a documented deliberate
exception, teardown_cmd.go:63–65). One forced duplication:
`internal/annotation/registry.go:103` hardcodes `"source", "no-source", "link"`
because pipeline imports annotation (cycle prevents importing the constants).

**Repo hygiene:** `docs/audit/` (10 files) and `docs/superpowers/`
(plans/handoffs/specs/audits) are git-tracked but absent from mkdocs nav —
violates CLAUDE.md "What to Commit" (working docs stay local).

## Verified-good (don't regress)

- Build, vet, full test suite all pass.
- Error format: `error:` + `hint:` convention working (config get/set, env get/set).
- Cobra "Did you mean?" suggestions work (`aply` → `apply`).
- Exit-code dichotomy (1 runtime / 2 usage) works.
- `--json` present on data commands; `list --json` exits 0.
- Help: command grouping, Long+Examples blocks, scoped flag visibility all good.
- Happy path works end-to-end once `.dagger` files exist.
- `dotd concepts` output clear and well-formatted.
