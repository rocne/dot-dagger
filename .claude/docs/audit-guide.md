# Code Audit Guide

This document captures the philosophy and methodology behind the recurring audits
in this project. Use it to scope and execute any future audit efficiently.

---

## The Three Principles

### 1. One canonical source for every value and behavior

Every fact — a string constant, a default path, a piece of logic — should live in
exactly one place. When the same fact appears in two places, a future change will
miss one of them.

**Violations to look for:**
- String literals repeated across files (`"source"`, `".dagger"`, `"warning:"`)
- Logic duplicated in two commands that serve the same purpose (e.g. two confirmation prompts)
- Default values computed inline rather than delegated to the resolution chain
- Helper functions re-implemented per package instead of shared

**Canonical examples in this codebase:**
- Action type strings → `pipeline.ActionSource` etc. (not `"source"`)
- Config filenames → `ecosystem.ConfigFile` (not `".dagger"`)
- Default paths → resolved once in `resolvePaths()`; commands read `cfg.*`
- Atomic YAML write → `fileutil.SaveYAML` (not duplicated in `config` and `env`)
- Confirmation prompt → `promptConfirm(out, r)` in `cmd/dotd/prompts.go`
- Compose filename stripping → `pipeline.ComposeFileName` (not inline in two places)

---

### 2. Every output goes through the right channel

There are three distinct output channels and each has a specific job. Bypassing
a channel breaks testability, routing, and consistent styling.

| Channel | Purpose | Mechanism |
|---------|---------|-----------|
| User-facing stdout | Status, previews, results | `cmd.OutOrStdout()` via cobra |
| Operational log | Debug/info/warn filtered by `--log-level` | `cfg.log.*` (charmbracelet/log) |
| Subprocess TTY | Interactive editors that need a real terminal | `c.Stdout = os.Stdout` (intentional exception) |

**Violations to look for:**
- `fmt.Fprintf(os.Stdout, ...)` or `fmt.Println(...)` in command code
- `fmt.Fprintf(os.Stderr, ...)` instead of `cfg.log.*`
- Status messages going to `cfg.log` when they should always be visible (not log-level-gated)
- Data output (lists, JSON) going to the logger instead of stdout

---

### 3. Call sites read like intent, not mechanism

A reader of a command function should see *what* it does, not *how* strings get
assembled. Every repeated assembly pattern is a candidate for a helper.

**Violations to look for:**
- `fmt.Fprintf(w, "%s %s\n", ui.OK("removed"), path)` — mechanism exposed
- `fmt.Fprintf(w, "%s\n", ui.Skip("cancelled"))` — wrapper needed
- `fmt.Fprintf(w, "\n%s\n", ui.Header("Next steps:"))` — pattern with shape
- Any multi-token construction that appears more than once

**Helpers that exist:**

| Helper | Renders as |
|--------|-----------|
| `ui.Warnf(w, fmt, ...)` | yellow `warning:` + message |
| `ui.Errf(w, fmt, ...)` | red `error:` + message |
| `ui.Hintf(w, fmt, ...)` | cyan `hint:` + message (replaced `Tipf` 2026-06-13) |
| `ui.OKf(w, fmt, ...)` | green whole line |
| `ui.Skipf(w, fmt, ...)` | faint whole line |
| `ui.Headerf(w, fmt, ...)` | bold section header with leading blank line |

cmd-layer helpers (cmd/dotd/format.go + errors.go, added 2026-06-13):
`plural(n, word)`, `bannerf(w, cmd, subtitle)`, `addJSONFlag(cmd, &b)`,
`keyArgs(n, usage, hint)`, `walkActive(cmd, cfg)` (the one pipeline
preamble). Shared internals: `fileutil.WriteAtomic` (only atomic writer),
`fileutil.ShellQuote` (only shell quoting), `ecosystem.GeneratedFileHeader`,
`predicate.SyntaxHelp`, `pipeline.BinPrefix`, setup's `sourceLineHeader`.

Use these. If a new pattern appears more than once, add a helper.

### Channel policy (ratified 2026-06-13, audit O1)

**Mutation results → stdout** via ui helpers/plain prints; never suppressed
by `--quiet` (the outcome of apply/adopt/unapply/teardown/setup/init is the
command's output, not a log line). **cfg.log carries diagnostics only**
(debug stage detail, warnings). Check-style reports keep their whole report
(detail + summary) on one channel.

### Error-wrap prefix rule (audit C3)

Each layer prefixes with its own identity: commands use the command name
(`adopt:`, `teardown:`), packages use the package name (`predicate:`,
`packages:`, `fileutil:`), pipeline-stage wraps in cmd use the stage name
(`walk:`, `order:`, `act:`). Don't restate a callee's prefix when wrapping.

---

## Audit Methodology

Run audits in this order — each layer builds on the previous.

### Pass 1 — Magic values

```
grep -rn '"source"\|"link"\|"compose"\|"no-source"' internal/
grep -rn '"\.dagger"\|"\.dotd\.yaml"' .
grep -rn 'ecosystem\.Default[A-Z]' cmd/dotd/  # outside resolvePaths
grep -rn 'os\.Getenv("DOTD_' cmd/dotd/        # after resolvePaths runs
grep -rn 'runtime\.GOOS\|os\.Getenv("SHELL"' . --include="*.go"
```

Any hit outside the resolution layer or the `getters.go` detectors is a violation.

### Pass 2 — Duplicated logic

Read every function longer than ~20 lines in `cmd/dotd/`. Ask: does any other
command do the same thing? Confirmation prompts, pipeline setup, path expansion,
and file-existence checks are common duplication sites.

### Pass 3 — Output routing

```
grep -rn 'fmt\.Fprintf(os\.Stdout\|fmt\.Println\|fmt\.Printf' cmd/dotd/
grep -rn 'fmt\.Fprintf(os\.Stderr' cmd/dotd/
```

Every hit should be either a legitimate subprocess exception (search for
`c.Stdout = os.Stdout` to see the known ones) or a bug.

### Pass 4 — Uncolorized / raw output

```
grep -rn 'fmt\.Fprintf(out\|fmt\.Fprintln(out\|fmt\.Fprintf(cmd\.Out' cmd/dotd/
```

For each result ask: is this a status/result line that should use a `ui.*` helper?
Data output (lists, key=value, JSON) is fine plain. Status messages (removed,
skipped, warning, error, cancelled, section headers) should use helpers.

### Pass 5 — Sprint helpers used inline unnecessarily

```
grep -rn 'ui\.OK(\|ui\.Skip(\|ui\.Missing(\|ui\.Wrong(\|ui\.Header(' cmd/dotd/
```

Any hit inside a `fmt.Fprintf` call is a candidate for replacement with the
corresponding `*f` helper — unless the colored text is embedded mid-sentence
or mixed with other colored tokens on the same line (those are fine to keep).

---

## Tool-name rule (ratified 2026-06-13)

Prose and help text may write `dotd` literally — readability wins, the name is
stable. Anything **written to disk or executed** (generated env.yaml content,
RC source lines, install scripts, file headers) must use `ecosystem.ToolD` /
`ecosystem.Name`. Never mix both forms in one string (the original sin:
setup's env template had `$(dotd get-os)` beside `$(%s get-hostname)`).

## Lessons from the 2026-06-13 audit

- **Tests can encode the bug.** Before fixing exit codes, prompts, or dry-run
  semantics, grep `*_test.go` AND `test/e2e/*.sh` for assertions of the old
  behavior (compose.sh asserted exit-0-on-stale; TestAct_Compose_DryRun
  asserted empty dry-run content; adopt.sh relied on non-TTY auto-accept).
- **Advertised vs honored flags**: cross-check `pathFlagOwners` in main.go
  against each command's RunE — a flag listed there but never read is a bug
  (teardown --dry-run).
- **Wrapped-when invariant**: every `when` expression source must be
  paren-wrapped before AND-joining (AND binds tighter than OR).
- **Dry-run = skip writes only.** The dry-run plan must equal the real plan;
  never skip reads in dry-run.

## What NOT to change

- **Data output** — `fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, v)` in `env`,
  `config`, `dag`, `list`, `package` commands. These are machine-parseable output;
  plain text is correct.
- **Subprocess I/O** — `c.Stdout = os.Stdout` in `config edit` and `env edit`.
  Interactive editors need a real TTY. This is the documented exception.
- **Logger calls** — `cfg.log.Infof(ui.Header("..."), ...)` mixing `ui.Header`
  into log messages is intentional; the logger handles its own formatting.
- **Prompt internals** — the `[default]` and `[Y/n]` prompt lines in `promptDefault`
  and `promptYN` are interactive UI chrome, not status output; leave them alone.
