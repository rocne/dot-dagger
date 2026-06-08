# UX and Help System Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix seven UX friction points: silence noisy usage output on errors, add `--debug` flag, improve `@when` syntax documentation, annotate shell expressions in `env show`, add `env path` and `env set` hints, and add `dotd concepts` reference command.

**Architecture:** All changes are in `cmd/dotd` and `internal/annotation`. Each task is independent — order matches dependency (SilenceUsage and --debug first since they affect every command; annotation changes next; env changes; concepts last).

**Tech Stack:** Go, Cobra (CLI), charmbracelet/huh (TUI prompts), charmbracelet/log (logging)

---

## File Map

| File | Change |
|------|--------|
| `cmd/dotd/main.go` | Add `debug bool` to `appConfig`; add `--debug` flag; extract `resolveLogLevel`; add `SilenceUsage: true` to root |
| `cmd/dotd/main_test.go` | Add `TestResolveLogLevel` |
| `internal/annotation/registry.go` | Update `WhenType.Description()` and `WhenType.Validate()` error |
| `internal/annotation/registry_test.go` | Update `TestValidate_WhenRejectsMissingEquals` and related to assert new error text |
| `cmd/dotd/annotate_cmd.go` | Print `t.Description()` as preamble for `KindText` fields |
| `cmd/dotd/env.go` | Update `newEnvShowCmd`; add `Long` to `newEnvSetCmd`; add `newEnvPathCmd` |
| `cmd/dotd/env_test.go` | Add `TestEnvShowExprAnnotation`, `TestEnvPathCmd` |
| `cmd/dotd/concepts_cmd.go` | New: `newConceptsCmd` |
| `.claude/TODO.md` | Add sub-topics expansion note |

---

## Task 0: Create feature branch

- [ ] **Create and switch to feature branch**

```bash
git checkout main && git pull
git checkout -b feature/claude-ux-help-improvements
```

---

## Task 1: Silence usage output on errors

**Files:**
- Modify: `cmd/dotd/main.go:59-64`

- [ ] **Add `SilenceUsage: true` to root command struct literal**

In `newRootCmd`, the root command is created at line 59. Add `SilenceUsage` alongside the existing `SilenceErrors`:

```go
root := &cobra.Command{
    Use:           ecosystem.ToolD,
    Short:         "Dotfiles manager — env resolution, DAG, symlinks, and init.sh generation",
    Version:       version,
    SilenceErrors: true,
    SilenceUsage:  true,
}
```

- [ ] **Build and verify manually**

```bash
go build ./cmd/dotd/... && ./dotd apply
```

Expected: error message only, no usage block printed beneath it.

- [ ] **Commit**

```bash
git add cmd/dotd/main.go
git commit -m "fix: silence usage output on command errors"
```

---

## Task 2: Add `--debug` flag

**Files:**
- Modify: `cmd/dotd/main.go` (appConfig struct, flag registration, configureLogger)
- Modify: `cmd/dotd/main_test.go` (add TestResolveLogLevel)

- [ ] **Write the failing test**

In `cmd/dotd/main_test.go`, add:

```go
func TestResolveLogLevel(t *testing.T) {
	cases := []struct {
		logLevel         string
		debug            bool
		logLevelExplicit bool
		quiet            bool
		want             string
	}{
		{"info", false, false, false, "info"},
		{"info", true, false, false, "debug"},  // --debug sets debug
		{"info", true, true, false, "info"},    // --log-level wins over --debug
		{"warn", true, true, false, "warn"},    // --log-level warn wins
		{"info", true, false, true, "error"},   // --quiet wins over --debug
		{"debug", false, true, false, "debug"}, // --log-level debug, no --debug flag
	}
	for _, c := range cases {
		got := resolveLogLevel(c.logLevel, c.debug, c.logLevelExplicit, c.quiet)
		if got != c.want {
			t.Errorf("resolveLogLevel(%q, debug=%v, explicit=%v, quiet=%v) = %q, want %q",
				c.logLevel, c.debug, c.logLevelExplicit, c.quiet, got, c.want)
		}
	}
}
```

- [ ] **Run test to confirm it fails**

```bash
go test ./cmd/dotd/... -run TestResolveLogLevel
```

Expected: `undefined: resolveLogLevel`

- [ ] **Add `debug bool` to `appConfig` struct**

In `main.go`, the `appConfig` struct currently ends at `quiet bool`. Add `debug` after `quiet`:

```go
type appConfig struct {
	files        string
	configPath   string
	envFile      string
	env          []string
	initFile     string
	linkRoot     string
	binDir       string
	generatedDir string
	dryRun       bool
	force        bool
	logLevel     string
	quiet        bool
	debug        bool
	log          *chlog.Logger
}
```

- [ ] **Extract `resolveLogLevel` and update `configureLogger`**

Replace the existing `configureLogger` function (currently at line ~249):

```go
// resolveLogLevel determines effective log level from three sources.
// Priority (highest wins): quiet > explicit --log-level > --debug > default.
func resolveLogLevel(logLevel string, debug bool, logLevelExplicit bool, quiet bool) string {
	level := logLevel
	if debug && !logLevelExplicit {
		level = "debug"
	}
	if quiet {
		level = "error"
	}
	return level
}

func configureLogger(cfg *config, cmd *cobra.Command) error {
	logLevelExplicit := cmd.Root().PersistentFlags().Changed("log-level")
	level := resolveLogLevel(cfg.logLevel, cfg.debug, logLevelExplicit, cfg.quiet)
	logger, err := dotlog.New(cmd.ErrOrStderr(), level)
	if err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	cfg.log = logger
	return nil
}
```

- [ ] **Register `--debug` persistent flag**

In `newRootCmd`, after the `--quiet` flag registration (line ~78):

```go
pf.BoolVar(&cfg.debug, "debug", false, "set log level to debug (overridden by --log-level)")
```

- [ ] **Run test to confirm it passes**

```bash
go test ./cmd/dotd/... -run TestResolveLogLevel
```

Expected: PASS

- [ ] **Run full suite**

```bash
go test ./...
```

Expected: all pass

- [ ] **Commit**

```bash
git add cmd/dotd/main.go cmd/dotd/main_test.go
git commit -m "feat: add --debug flag; extract resolveLogLevel"
```

---

## Task 3: `@when` syntax description and validate error

**Files:**
- Modify: `internal/annotation/registry.go` (WhenType.Description, WhenType.Validate)
- Modify: `internal/annotation/registry_test.go` (update error-text assertions)

- [ ] **Update `WhenType.Validate` tests first to assert new error text**

The existing tests at lines 49–68 of `registry_test.go` check that errors are non-nil but don't assert the message. Add one test that asserts the hint is present:

```go
func TestValidate_WhenErrorIncludesHint(t *testing.T) {
	err := WhenType{}.Validate("invalid")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "AND/OR") {
		t.Errorf("error message missing AND/OR hint: %q", msg)
	}
	if !strings.Contains(msg, "comma separates") {
		t.Errorf("error message missing comma hint: %q", msg)
	}
}
```

Add `"strings"` to imports in `registry_test.go` if not present.

- [ ] **Run new test to confirm it fails**

```bash
go test ./internal/annotation/... -run TestValidate_WhenErrorIncludesHint
```

Expected: FAIL — current error message has no hint.

- [ ] **Update `WhenType.Description()` in `registry.go`**

Replace the existing single-line `Description()`:

```go
func (WhenType) Description() string {
	return "Condition for when this file is active.\n\n" +
		"  key=value              single condition       os=macos\n" +
		"  key=v1,v2             match any value        os=macos,linux\n" +
		"  expr AND expr         both must match        os=macos AND context=work\n" +
		"  expr OR expr          either matches         os=macos OR os=linux\n" +
		"  (expr)                grouping               (os=macos OR os=linux) AND context=work\n\n" +
		"Comma separates multiple values for ONE key. Use AND to join two conditions."
}
```

- [ ] **Update `WhenType.Validate()` in `registry.go`**

```go
func (WhenType) Validate(s string) error {
	idx := strings.IndexByte(s, '=')
	if idx < 1 || idx >= len(s)-1 {
		return fmt.Errorf(
			"@when: expected format key=value or key=v1,v2 (got %q)\nhint: use AND/OR to join conditions; comma separates multiple values for one key (e.g. os=macos,linux)",
			s,
		)
	}
	return nil
}
```

- [ ] **Run annotation tests**

```bash
go test ./internal/annotation/... -v
```

Expected: all pass including new hint test.

- [ ] **Commit**

```bash
git add internal/annotation/registry.go internal/annotation/registry_test.go
git commit -m "feat: add @when syntax reference to description and validate error hint"
```

---

## Task 4: `annotate_cmd.go` — KindText preamble

**Files:**
- Modify: `cmd/dotd/annotate_cmd.go:151-158`

- [ ] **Update KindText case to print description as preamble**

Find the `KindText` case (currently line ~151):

```go
case annotation.KindText:
    prefill := ""
    if len(entries) == 1 {
        prefill = entries[0].Value
    }
    val, err := promptInput(cmd, t.Label(), t.Description()+"  (clear the field to remove)", prefill, t.Validate)
```

Replace with:

```go
case annotation.KindText:
    prefill := ""
    if len(entries) == 1 {
        prefill = entries[0].Value
    }
    fmt.Fprintln(out, t.Description())
    val, err := promptInput(cmd, t.Label(), "(enter value, or clear to remove)", prefill, t.Validate)
```

- [ ] **Build and verify manually**

```bash
go build ./cmd/dotd/...
```

Then run: `dotd annotate <any-file-in-dotfiles-repo>` and select "When" — the multi-line syntax block should appear before the input prompt.

- [ ] **Run full test suite**

```bash
go test ./...
```

Expected: all pass (this change has no automated test; visual verification above is sufficient).

- [ ] **Commit**

```bash
git add cmd/dotd/annotate_cmd.go
git commit -m "feat: print @when syntax block as preamble in annotate wizard"
```

---

## Task 5: `env show` — shell expression annotation

**Files:**
- Modify: `cmd/dotd/env.go` (newEnvShowCmd)
- Create: `cmd/dotd/env_test.go`

- [ ] **Write the failing test**

Create `cmd/dotd/env_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvShowExprAnnotation(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("greeting: $(echo hello)\nplain: world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config{envFile: envFile}
	cmd := newEnvShowCmd(cfg)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	// shell expression value gets annotated
	if !strings.Contains(out, "greeting=hello") {
		t.Errorf("want evaluated value in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[$(echo hello)]") {
		t.Errorf("want shell expr annotation, got:\n%s", out)
	}

	// plain value has no annotation
	if !strings.Contains(out, "plain=world") {
		t.Errorf("want plain value in output, got:\n%s", out)
	}
	if strings.Contains(out, "[$(") && strings.Contains(out, "plain") {
		t.Errorf("plain value should not have annotation, got:\n%s", out)
	}
}
```

- [ ] **Run test to confirm it fails**

```bash
go test ./cmd/dotd/... -run TestEnvShowExprAnnotation
```

Expected: FAIL — current `env show` doesn't annotate shell expressions.

- [ ] **Update `newEnvShowCmd` in `env.go`**

Add `"strings"` to the import block in `env.go` if not present. Replace `newEnvShowCmd`:

```go
func newEnvShowCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all resolved env key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return err
			}
			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(resolved))
			for k := range resolved {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rawVal := raw[k]
				if strings.HasPrefix(rawVal, "$(") && strings.HasSuffix(rawVal, ")") {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\t[%s]\n", k, resolved[k], rawVal)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, resolved[k])
				}
			}
			return nil
		},
	}
}
```

- [ ] **Run test to confirm it passes**

```bash
go test ./cmd/dotd/... -run TestEnvShowExprAnnotation
```

Expected: PASS

- [ ] **Run full suite**

```bash
go test ./...
```

Expected: all pass

- [ ] **Commit**

```bash
git add cmd/dotd/env.go cmd/dotd/env_test.go
git commit -m "feat: annotate shell expressions in env show output"
```

---

## Task 6: `env set` — shell expression hint

**Files:**
- Modify: `cmd/dotd/env.go` (newEnvSetCmd)

- [ ] **Add `Long` description to `newEnvSetCmd`**

```go
func newEnvSetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: fmt.Sprintf("Set a key in %s", ecosystem.EnvFileName),
		Long: fmt.Sprintf(`Set a key in %s.

To store a shell expression that evaluates at runtime, use single quotes
to prevent the shell from expanding it:

  dotd env set os '$(dotd get-os)'
  dotd env set hostname '$(hostname)'

Values stored as $(…) are evaluated each time dotd runs.`, ecosystem.EnvFileName),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := envYamlPath(cfg)
			raw, err := env.Load(path)
			if err != nil {
				return err
			}
			raw[args[0]] = args[1]
			return env.Save(path, raw)
		},
	}
}
```

- [ ] **Build and verify**

```bash
go build ./cmd/dotd/... && ./dotd env set --help
```

Expected: Long description with shell expression quoting examples is shown.

- [ ] **Run full suite**

```bash
go test ./...
```

- [ ] **Commit**

```bash
git add cmd/dotd/env.go
git commit -m "feat: add shell expression hint to env set --help"
```

---

## Task 7: `dotd env path` subcommand

**Files:**
- Modify: `cmd/dotd/env.go` (add newEnvPathCmd, register in newEnvCmd)
- Modify: `cmd/dotd/env_test.go` (add TestEnvPathCmd)

- [ ] **Write the failing test**

In `cmd/dotd/env_test.go`, add:

```go
func TestEnvPathCmd(t *testing.T) {
	cfg := &config{envFile: "/custom/path/env.yaml"}
	cmd := newEnvPathCmd(cfg)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "/custom/path/env.yaml" {
		t.Errorf("got %q, want %q", got, "/custom/path/env.yaml")
	}
}
```

- [ ] **Run test to confirm it fails**

```bash
go test ./cmd/dotd/... -run TestEnvPathCmd
```

Expected: `undefined: newEnvPathCmd`

- [ ] **Implement `newEnvPathCmd`**

In `env.go`, add after `newEnvSetCmd`:

```go
func newEnvPathCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the path to env.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), cfg.envFile)
			return nil
		},
	}
}
```

- [ ] **Register in `newEnvCmd`**

In `newEnvCmd`, add `newEnvPathCmd(cfg)` to `cmd.AddCommand(...)`:

```go
cmd.AddCommand(
    newEnvShowCmd(cfg),
    newEnvGetCmd(cfg),
    newEnvSetCmd(cfg),
    newEnvEditCmd(cfg),
    newEnvDiffCmd(cfg),
    newEnvPathCmd(cfg),
)
```

- [ ] **Run test to confirm it passes**

```bash
go test ./cmd/dotd/... -run TestEnvPathCmd
```

Expected: PASS

- [ ] **Run full suite**

```bash
go test ./...
```

- [ ] **Commit**

```bash
git add cmd/dotd/env.go cmd/dotd/env_test.go
git commit -m "feat: add dotd env path subcommand"
```

---

## Task 8: `dotd concepts` command

**Files:**
- Create: `cmd/dotd/concepts_cmd.go`
- Modify: `cmd/dotd/main.go` (register command)

- [ ] **Create `cmd/dotd/concepts_cmd.go`**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const conceptsText = `dotd — concepts reference
═════════════════════════

PIPELINE
  dotd processes dotfiles in five stages:

  walk      Traverse the dotfiles repo, read .dagger annotation files and
            file headers to build a list of nodes.
  filter    Evaluate each node's @when predicate against the resolved env
            map. Nodes whose condition is false are excluded.
  order     Topological sort: nodes listed in @after dependencies come first.
  act       Execute actions: create symlinks, record sourced files, assemble
            compose targets.
  init.sh   Regenerate the shell init file from all sourced nodes.

PREDICATES (@when)
  @when controls whether a node is active. Syntax:

    key=value              single condition       os=macos
    key=v1,v2             match any value        os=macos,linux
    expr AND expr         both must match        os=macos AND context=work
    expr OR expr          either matches         os=macos OR os=linux
    (expr)                grouping               (os=macos OR os=linux) AND context=work

  Comma separates multiple values for ONE key.
  Use AND/OR to join two separate conditions.

ANNOTATIONS
  Written as comments in file headers (e.g. # @when(os=macos)).
  Managed interactively with: dotd annotate <file>

    @when(expr)        Condition for activation (see PREDICATES above)
    @action(type)      How dotd processes this file: source, no-source, link
    @after(name)       Logical name this file must load after
    @name(name)        Override the logical name used in the dependency graph
    @require(pkg)      Package that must be installed (blocks activation)
    @request(pkg)      Package to install if missing (non-blocking)
    @disable           Exclude this file from all processing

ENV.YAML
  A flat YAML file mapping string keys to string values:

    os: $(dotd get-os)
    hostname: $(hostname)
    context: work

  Shell expressions: values matching $(…) are evaluated via sh -c each
  time dotd runs. Use single quotes to store them without shell expansion:

    dotd env set os '$(dotd get-os)'

  Resolution order (highest priority wins):
    1. --env flags        e.g. --env context=work
    2. DOTD_* shell vars  e.g. DOTD_CONTEXT=work
    3. env.yaml values

  Commands: dotd env show | get | set | edit | diff | path

DIRECTORY NAMING
  dotd interprets directory and file name prefixes:

    dot-bashrc      links/names as .bashrc  (leading dot added)
    nosync-work/    strips "nosync-"        (avoids sync-tool ignore rules)
    shellrc.d/      compose target          (contents assembled into shellrc)
`

func newConceptsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "concepts",
		Short: "Print a reference of dotd concepts and syntax",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), conceptsText)
			return nil
		},
	}
}
```

- [ ] **Register `newConceptsCmd` in `main.go`**

In `newRootCmd`, find where other commands are added. Add:

```go
root.AddCommand(newConceptsCmd())
```

alongside the other `root.AddCommand(...)` calls.

- [ ] **Build and verify**

```bash
go build ./cmd/dotd/... && ./dotd concepts
```

Expected: multi-section reference printed to stdout.

- [ ] **Run full suite**

```bash
go test ./...
```

- [ ] **Commit**

```bash
git add cmd/dotd/concepts_cmd.go cmd/dotd/main.go
git commit -m "feat: add dotd concepts reference command"
```

---

## Task 9: Update `.claude/TODO.md`

**Files:**
- Modify: `.claude/TODO.md`

- [ ] **Add sub-topics TODO**

Open `.claude/TODO.md` and add:

```
- [ ] dotd concepts: add sub-topic routing (dotd concepts when, dotd concepts env, etc.)
      once the flat version is validated with users
```

- [ ] **Commit**

```bash
git add .claude/TODO.md
git commit -m "chore: note dotd concepts sub-topic expansion as future work"
```

---

## Task 10: Open PR

- [ ] **Verify baseline**

```bash
go test ./...
```

Expected: all 15 packages pass.

- [ ] **Push and open PR**

```bash
git push -u origin feature/claude-ux-help-improvements
gh pr create \
  --title "feat: UX and help system improvements" \
  --body "$(cat <<'EOF'
## Summary

Seven UX friction points fixed based on real usage transcript:

- **SilenceUsage** — errors no longer buried under full usage block
- **`--debug` flag** — standard shorthand for `--log-level debug`; `--log-level` overrides if both set
- **`@when` syntax** — multi-line syntax reference in annotate wizard preamble; validate error includes AND/OR hint
- **`env show`** — shell expressions annotated inline: `os=macos  [$(dotd get-os)]`
- **`env set`** — `--help` explains single-quote escaping for shell expressions
- **`env path`** — new subcommand prints env.yaml location
- **`dotd concepts`** — new command prints pipeline, predicate, annotation, env, and naming reference

## Test plan

- [ ] `go test ./...` passes
- [ ] `dotd apply` on bad config: error only, no usage block
- [ ] `dotd --debug apply`: debug log lines appear
- [ ] `dotd annotate <file>` → When: syntax block appears before prompt
- [ ] `dotd env show` with shell-expr env.yaml: `[$(expr)]` visible
- [ ] `dotd env set --help`: single-quote hint visible
- [ ] `dotd env path`: prints env.yaml path
- [ ] `dotd concepts`: full reference printed

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
