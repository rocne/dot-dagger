# dotd CLI UX Audit — 2026-06-08

Audited root + all 16 subcommands + error paths + I/O conventions against the
cli-ux skill checklist.

Overall: solid. Command grouping, TTY detection, NO_COLOR, completion, env-var
discipline all well done. Main gaps: error message format inconsistency, sparse
`--help` content, generic cobra arg errors, and `--json` only on one command.

---

## Findings

### High priority

**1. Two `error:` prefix styles coexist.**
`cmd/dotd/main.go:31` prints `Error: %s` (capital E). Meanwhile
`internal/ui/ui.go:23` `Errf` writes `error:` (lowercase) for in-command
warnings. Pick one — lowercase matches existing `ui` package and the cli-ux
skill convention.
```go
fmt.Fprintf(os.Stderr, "error: %s\n", err)
```

**2. `Hint:` should be lowercase + own line.**
`main.go:377`, `filter_prompt.go:63` use `Hint:`. Convention: `hint:` on its
own line directly under `error:`. Today, `annotateKeyError` embeds `\n\nHint:`
inside the error string which the root handler then prefixes with `Error: `.
Cleaner if root handler renders both:
```
error: <msg>
hint:  <msg>
```
Could expose a `HintError` interface (`Hint() string`) that errors implement,
or use a typed error.

**3. Sparse help text.**
Only `apply`, `check`, `list`, `env set`, `bundle` have `Long` descriptions.
Missing on: `adopt`, `annotate`, `init`, `setup`, `teardown`, `unapply`,
`dag check`, `compose check`, `compose list`, `package list`/`check`/`generate`,
all `config` subcommands, `env show`/`get`/`diff`/`path`/`edit`, `concepts`,
`completion`. Every command needs at least an `Examples:` block.

**4. Generic cobra arg errors leak.**
Bad invocations get raw cobra strings, no hint:
```
$ dotd config get
Error: accepts 1 arg(s), received 0
$ dotd config get bogus
Error: config: unknown key "bogus"
```
For `config get`/`set` and `env get`/`set`: catch unknown-key, list valid
keys (`dotcfg.Keys` is available). For missing-arg, show usage example.
Replace `cobra.ExactArgs(N)` with custom validator that returns a richer
error.

**5. Cobra suggestions disabled.**
`dotd version` → `Error: unknown command "version"`. Cobra has `Did you
mean…?` built-in. Confirm `SuggestionsMinimumDistance` is set and
`DisableSuggestions` is false. Easy win.

### Medium priority

**6. Global flag clutter on subcommand help.**
Every subcommand shows all 11 persistent flags. `dotd config get --help`
shows `--bin-dir`, `--generated-dir`, `--init-file`, `--link-root` — none
relevant. Options: scope path flags to commands that use them (refactor
`resolvePaths` to be lazy/per-command) or split the global flag table by
relevance using `cobra.Command.SetUsageTemplate`.

**7. `--json` only on `list`.**
Data commands without machine-readable output: `config show`, `env show`,
`env diff`, `dag check`, `package list`, `package check`, `compose list`,
`compose check`. Each invents its own text format. Scripting hostile. Add
`--json` to every data-producing command.

**8. `--quiet` doesn't quiet data prints.**
`main.go:260` maps `--quiet` to log level `error`. But commands print data
via `fmt.Fprintf(cmd.OutOrStdout(), …)` independent of the logger —
e.g. `apply --dry-run`'s `# link …` lines, `compose check`, `env show`.
Either redefine `--quiet` as logs-only (document it) or thread it through
data printers (suppress non-essential lines, keep machine-readable output).

**9. `setup` cannot be scripted.**
`adopt`/`unapply`/`teardown` have `-y/--yes`. `setup` is fully interactive
— no `--non-interactive` or `--yes`, and no flag-based input for the
values it asks. Means automation cannot bootstrap a fresh machine. Add
`--non-interactive` that accepts shown defaults (already supported
internally via `nonInteractive` in `promptDefault`).

**10. `env show` mixed format.**
Some lines `KEY=val`, some `KEY=val\t[$(…)]`. Tab field is invisible;
piped consumers see one column or two unpredictably. Either always show
source column (consistent) or move source info behind `--verbose`/`--json`.

### Low priority

**11. `apply --dry-run` uses `#` prefix.**
Lines look like shell comments (`# link foo → bar`). Confusing if piped
to `sh`. Either drop the `#` and emit a canonical format, or use
`dry-run:` prefix.

**12. Color semantic clash.**
`ui.Missing` and `ui.Wrong` both yellow. `check` output mixes them:
`2 ok, 1 missing, 1 wrong` — `missing` and `wrong` indistinguishable.
Make `Wrong` red (it's a divergent symlink — that's worse than absent).

**13. `dotd init` error style off.**
`init_cmd.go:37`:
```go
return fmt.Errorf("no config found — run 'dotd setup' first")
```
Becomes `Error: no config found — run 'dotd setup' first`. Has the
hint inline but loses the structured `error:`/`hint:` split. Match the
new convention once decided.

**14. Exit code semantics.**
Skill: exit 1 = runtime, exit 2 = usage. `main.go:33` always
`os.Exit(1)`. Cobra's `RunE` doesn't distinguish. Standard practice for
cobra apps, but scripts can't differentiate "your invocation was wrong"
from "the operation failed." Optional.

**15. `concepts` not surfaced.**
Lives under "Additional Commands". Should be group-tagged (Configuration?
or its own "Reference:" group) so users discover it. Also `dotd --help`
could end with: `For concepts, run: dotd concepts`.

**16. No README install snippet for completion.**
Implementation is there. Need user-facing install lines per shell.
cli-ux skill has the template.

**17. `unapply` is non-standard.**
Common verbs: `remove`, `clean`, `revert`. Aliasing
(`Aliases: []string{"remove"}`) costs nothing. Optional.

---

## Positives — don't regress

- Command grouping (Core/Configuration/Advanced) — clear visual organization.
- `--all` reveals hidden internal helpers — opt-in discoverability.
- `--debug` as `--log-level=debug` shorthand — friendly.
- TTY detection via `Fd()` interface (`prompts.go:36`) — handles
  `strings.Reader` correctly.
- huh accessible mode auto-switches in non-TTY — tests and CI work.
- `NO_COLOR` honored via `fatih/color`.
- `errUserAborted` sentinel → clean `cancelled` exit, no stack trace.
- `--env key=value` repeatable, format validated.
- `DOTD_` env-var discipline, documented in `CLAUDE.md`.
- Canonical `resolvePaths` chain — exemplary architecture.
- Errors → stderr, data → stdout (consistent).
- `--dry-run` consistent across mutating commands.
- Completion command for bash/zsh/fish/powershell — present.

---

## Remediation Plan

Batched into 5 PRs, smallest/highest-leverage first.

### PR 1 — Error format unification
Fixes #1, #2, #13.

Scope:
- Lowercase `error:` in root handler (`main.go:31`).
- Introduce `HintError` interface or typed error so root handler renders
  `hint:` on its own line.
- Convert `annotateKeyError` (`main.go:374`) to return a `HintError`
  instead of embedding `\n\nHint:` in the message string.
- Convert `init_cmd.go:37` "no config found" to a `HintError` with
  `hint: run 'dotd setup' first`.
- Audit other `fmt.Errorf("... — ...")` patterns that smuggle hints into
  message strings; convert to `HintError`.
- Update tests that assert on error string format.

Small, mechanical, high signal. Touches root handler and ~5 call sites.

### PR 2 — Help text + arg errors
Fixes #3, #4, #5.

Scope:
- Add `Long` + `Examples` to every command currently missing them.
- Custom `cobra.PositionalArgs` validator for `config get`/`set` and
  `env get`/`set` that:
  - Lists valid keys on unknown-key error.
  - Shows usage example on missing-arg.
- Enable cobra suggestions (verify `DisableSuggestions=false` and tune
  `SuggestionsMinimumDistance`).
- Largest in line count but mostly mechanical.

### PR 3 — `--json` everywhere
Fixes #7.

Scope: add `--json` to `config show`, `env show`, `env diff`, `dag check`,
`package list`, `package check`, `compose list`, `compose check`. Mirror
the `list_cmd.go` pattern (struct + JSON branch).

### PR 4 — `setup --non-interactive` + `--quiet` semantics
Fixes #8, #9.

Scope:
- Add `--non-interactive` to `setup` that accepts shown defaults
  throughout the wizard.
- Decide quiet semantics (logs-only vs full silence) and document; if
  full silence, thread the flag through data printers.

### PR 5 — Polish
Fixes #6, #10, #11, #12, #15, #17.

Scope: per-command flag visibility, `env show` format, dry-run prefix,
color palette tweak, `concepts` discoverability, `unapply` alias.

Optional/deferred: #14 (exit codes), #16 (README completion install).
