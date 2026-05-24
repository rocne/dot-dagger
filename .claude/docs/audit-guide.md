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
| `ui.Tipf(w, fmt, ...)` | cyan `tip:` + message |
| `ui.OKf(w, fmt, ...)` | green whole line |
| `ui.Skipf(w, fmt, ...)` | faint whole line |
| `ui.Headerf(w, fmt, ...)` | bold section header with leading blank line |

Use these. If a new pattern appears more than once, add a helper.

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
