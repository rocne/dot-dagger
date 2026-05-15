# CLI Completeness + Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill CLI gaps (`dotd env diff`, `dotd package list`), reduce duplication in `runApply`/`runCheck`, unhide stable subcommands, update the spec to match implementation, and add missing tests for `internal/setup` and `internal/ecosystem`.

**Architecture:** All CLI changes live in `cmd/dotd/`. The shared pipeline runner is a private helper in `main.go`. New subcommands follow the existing pattern: one `new*Cmd` constructor per command, registered in the parent `new*Cmd`. Tests follow the `run(t, args...)` helper pattern in `main_test.go`.

**Tech Stack:** Go, Cobra (`github.com/spf13/cobra`), standard `testing` package.

**Out of scope:** `dotd adopt` migration — that is a separate feature.

---

## File Map

| File | Change |
|------|--------|
| `cmd/dotd/main.go` | Add `pipelineRun` struct + `runPipeline` helper; simplify `runApply` and `runCheck` |
| `cmd/dotd/env.go` | Add `newEnvDiffCmd`; register it in `newEnvCmd` |
| `cmd/dotd/package.go` | Add `newPackageListCmd`; register it in `newPackageCmd`; remove `Hidden: true` |
| `cmd/dotd/compose_cmd.go` | Remove `Hidden: true` from `newComposeCmd` |
| `cmd/dotd/dag_cmd.go` | Remove `Hidden: true` from `newDagCmd` |
| `cmd/dotd/main_test.go` | Add tests for `env diff` and `package list` |
| `internal/setup/setup_test.go` | New file — unit tests for `setup.Scaffold` |
| `internal/ecosystem/ecosystem_test.go` | New file — unit tests for path resolution functions |
| `.claude/docs/spec/cli.md` | Update to match actual implementation |

---

## Task 1: Extract shared pipeline runner

**Files:**
- Modify: `cmd/dotd/main.go`

`runApply` and `runCheck` each duplicate the same walk → validate → filter → order → act sequence (~50 lines). Extract it.

- [ ] **Step 1: Write the failing test (regression guard)**

These tests already exist. Run them to establish baseline:

```
go test ./cmd/dotd/... -run 'TestApply|TestCheck|TestList' -v
```

Expected: all pass (or none exist yet — that's fine, existing suite still runs).

- [ ] **Step 2: Add `pipelineRun` struct and `runPipeline` to `main.go`**

Add after the `buildActOptions` function (around line 241):

```go
type pipelineRun struct {
	allCount    int
	activeCount int
	result      *pipeline.ActResult
}

func runPipeline(cfg *config, dryRun bool) (*pipelineRun, error) {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return nil, annotateKeyError(err)
	}

	nodes, err := pipeline.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}

	if err := pipeline.ValidateNodes(nodes); err != nil {
		return nil, err
	}

	active, err := pipeline.Filter(nodes, resolved)
	if err != nil {
		return nil, annotateKeyError(err)
	}

	ordered, err := pipeline.Order(active)
	if err != nil {
		return nil, fmt.Errorf("order: %w", err)
	}

	actOpts, err := buildActOptions(cfg, dryRun)
	if err != nil {
		return nil, err
	}
	res, err := pipeline.Act(ordered, actOpts)
	if err != nil {
		return nil, fmt.Errorf("act: %w", err)
	}

	return &pipelineRun{
		allCount:    len(nodes),
		activeCount: len(active),
		result:      res,
	}, nil
}
```

- [ ] **Step 3: Replace `runApply` body**

Replace the body of `runApply` with:

```go
func runApply(cmd *cobra.Command, cfg *config) error {
	run, err := runPipeline(cfg, false)
	if err != nil {
		return err
	}
	cfg.log.Debugf("%s %d active / %d total", ui.Header("filter:"), run.activeCount, run.allCount)

	if cfg.dryRun {
		for _, lnk := range run.result.Links {
			fmt.Fprintf(cmd.OutOrStdout(), "# link %s %s %s\n", lnk.Src, ui.Arrow("→"), lnk.Dest)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# would write %s (%d sourced nodes)\n", cfg.initFile, len(run.result.Sourced))
		return nil
	}

	cfg.log.Infof("%s %d symlinks applied", ui.Header("links:"), len(run.result.Links))

	if err := pipeline.Generate(cfg.initFile, run.result.Sourced); err != nil {
		return fmt.Errorf("generate init.sh: %w", err)
	}
	cfg.log.Infof("%s wrote %s (%d nodes)", ui.Header("init.sh:"), cfg.initFile, len(run.result.Sourced))
	return nil
}
```

- [ ] **Step 4: Replace `runCheck` body**

```go
func runCheck(cmd *cobra.Command, cfg *config) error {
	run, err := runPipeline(cfg, true)
	if err != nil {
		return err
	}
	cfg.log.Infof("%s %d active / %d total", ui.Header("filter:"), run.activeCount, run.allCount)

	var hasIssues bool

	var ok, missing, wrong int
	for _, lnk := range run.result.Links {
		target, lerr := os.Readlink(lnk.Dest)
		if errors.Is(lerr, fs.ErrNotExist) {
			missing++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "missing")
		} else if lerr != nil {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "not-a-symlink")
		} else if target != lnk.Src {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "wrong-target", "want", lnk.Src, "got", target)
		} else {
			ok++
		}
	}
	cfg.log.Infof("%s %d ok, %d missing, %d wrong", ui.Header("symlinks:"), ok, missing, wrong)

	if _, serr := os.Stat(cfg.initFile); errors.Is(serr, fs.ErrNotExist) {
		cfg.log.Warn(cfg.initFile, "state", "missing")
		hasIssues = true
	} else {
		cfg.log.Infof("%s %s present (%d sourced nodes)", ui.Header("init.sh:"), cfg.initFile, len(run.result.Sourced))
	}

	if hasIssues {
		cmd.SilenceErrors = true
		return errors.New("check: issues found")
	}
	return nil
}
```

- [ ] **Step 5: Verify tests still pass**

```
go test ./cmd/dotd/... -v
```

Expected: all tests that passed before still pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/main.go
git commit -m "refactor(cmd): extract runPipeline to deduplicate apply/check pipeline setup"
```

---

## Task 2: `dotd env diff` command

**Files:**
- Modify: `cmd/dotd/env.go`
- Modify: `cmd/dotd/main_test.go`

`dotd env diff` shows which keys in `env.yaml` contribute values that would not exist (or differ) if env.yaml were absent. It compares: resolved values with env.yaml vs resolved values from `DOTD_*` shell vars alone.

- [ ] **Step 1: Write the failing tests**

Add to `cmd/dotd/main_test.go`:

```go
// --- dotd env diff ---

func TestEnvDiffShowsOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure no DOTD_CONTEXT in environment.
	t.Setenv("DOTD_CONTEXT", "")

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "context") {
		t.Errorf("expected 'context' in diff output, got %q", out)
	}
}

func TestEnvDiffEmptyFile(t *testing.T) {
	out, err := run(t, "env", "diff",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides', got %q", out)
	}
}

func TestEnvDiffShellVarMatches(t *testing.T) {
	// When env.yaml value equals DOTD_* value, no diff line shown.
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTD_CONTEXT", "work")

	out, err := run(t, "env", "diff",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env diff error = %v", err)
	}
	if !strings.Contains(out, "no overrides") {
		t.Errorf("expected 'no overrides' when values match, got %q", out)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./cmd/dotd/... -run TestEnvDiff -v
```

Expected: `FAIL` — command not registered.

- [ ] **Step 3: Add `newEnvDiffCmd` to `env.go`**

Add these imports to `env.go` if not already present: `"sort"` (already there), `"os"` (already there), `"strings"` (already there).

Add the function after `newEnvShowCmd`:

```go
func newEnvDiffCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show env.yaml keys that override shell-detected values",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}
			expanded, err := env.Expand(raw)
			if err != nil {
				return err
			}
			shellVars := env.ShellVars(os.Environ())

			keys := make([]string, 0, len(expanded))
			for k := range expanded {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			var any bool
			for _, k := range keys {
				envVal := expanded[k]
				shellVal, inShell := shellVars[k]
				if inShell && envVal == shellVal {
					continue
				}
				if inShell {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %q (shell) → %q (env.yaml)\n", k, shellVal, envVal)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: (unset) → %q (env.yaml)\n", k, envVal)
				}
				any = true
			}
			if !any {
				fmt.Fprintln(cmd.OutOrStdout(), "no overrides — env.yaml values match shell")
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Register in `newEnvCmd`**

In `newEnvCmd`, add `newEnvDiffCmd(cfg)` to the `AddCommand` call:

```go
func newEnvCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Inspect and modify env.yaml",
	}
	cmd.AddCommand(
		newEnvShowCmd(cfg),
		newEnvGetCmd(cfg),
		newEnvSetCmd(cfg),
		newEnvEditCmd(cfg),
		newEnvDiffCmd(cfg),
	)
	return cmd
}
```

- [ ] **Step 5: Run tests**

```
go test ./cmd/dotd/... -run TestEnvDiff -v
```

Expected: all three `TestEnvDiff*` tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/env.go cmd/dotd/main_test.go
git commit -m "feat(cmd): add dotd env diff command"
```

---

## Task 3: `dotd package list` command

**Files:**
- Modify: `cmd/dotd/package.go`
- Modify: `cmd/dotd/main_test.go`

`dotd package list` lists all packages referenced in active nodes (from `@require` / `@request` annotations) without checking install status.

- [ ] **Step 1: Write the failing test**

Add to `cmd/dotd/main_test.go`:

```go
// --- dotd package list ---

func TestPackageListEmpty(t *testing.T) {
	dir := t.TempDir()
	// Write empty packages.yaml so loadRegistry doesn't error.
	if err := os.WriteFile(filepath.Join(dir, "packages.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, "package", "list",
		"--env-file", emptyEnvFile(t),
		"--files", dir,
	)
	if err != nil {
		t.Fatalf("package list error = %v", err)
	}
}

func TestPackageListShowsAnnotations(t *testing.T) {
	dir := t.TempDir()
	// One shellrc file with @require annotation.
	shellrc := filepath.Join(dir, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "base.sh"), []byte("# @require(ripgrep)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "package", "list",
		"--env-file", emptyEnvFile(t),
		"--files", dir,
	)
	if err != nil {
		t.Fatalf("package list error = %v", err)
	}
	if !strings.Contains(out, "ripgrep") {
		t.Errorf("expected 'ripgrep' in output, got %q", out)
	}
	if !strings.Contains(out, "require") {
		t.Errorf("expected 'require' in output, got %q", out)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./cmd/dotd/... -run TestPackageList -v
```

Expected: `FAIL` — command not registered.

- [ ] **Step 3: Add `newPackageListCmd` to `package.go`**

Add after `newPackageCheckCmd`:

```go
func newPackageListCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all packages referenced in active nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, err := pipeline.Walk(cfg.files)
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
			active, err := pipeline.Filter(nodes, resolved)
			if err != nil {
				return annotateKeyError(err)
			}

			reqs := collectPackageRequests(active)
			seen := map[string]bool{}
			for _, r := range reqs {
				if seen[r.Package] {
					continue
				}
				seen[r.Package] = true
				kind := "request"
				if r.Hard {
					kind = "require"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s\n", r.Package, kind)
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Register in `newPackageCmd`**

```go
func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "package",
		Short:  "Package management — filtered by active predicates",
		Hidden: true,
	}
	cmd.AddCommand(
		newPackageCheckCmd(cfg),
		newPackageGenerateCmd(cfg),
		newPackageListCmd(cfg),
	)
	return cmd
}
```

- [ ] **Step 5: Run tests**

```
go test ./cmd/dotd/... -run TestPackageList -v
```

Expected: both `TestPackageList*` tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/package.go cmd/dotd/main_test.go
git commit -m "feat(cmd): add dotd package list command"
```

---

## Task 4: Unhide stable subcommands + update spec

**Files:**
- Modify: `cmd/dotd/package.go`
- Modify: `cmd/dotd/compose_cmd.go`
- Modify: `cmd/dotd/dag_cmd.go`
- Modify: `.claude/docs/spec/cli.md`

`package`, `compose`, and `dag` commands are hidden but stable. The spec describes commands that no longer exist (`dotd link apply/check/remove`) and misnames others (`dotd setup` → `dotd init`, `dotd files list` → `dotd list`).

- [ ] **Step 1: Remove `Hidden: true` from `newPackageCmd`**

In `cmd/dotd/package.go`, change:

```go
func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package management — filtered by active predicates",
	}
```

(Remove `Hidden: true` line.)

- [ ] **Step 2: Remove `Hidden: true` from `newComposeCmd`**

In `cmd/dotd/compose_cmd.go`, change:

```go
func newComposeCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Manage compose targets (assembled fragment files)",
	}
```

- [ ] **Step 3: Remove `Hidden: true` from `newDagCmd`**

In `cmd/dotd/dag_cmd.go`, change:

```go
func newDagCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Inspect the dotfile dependency graph",
	}
```

- [ ] **Step 4: Run tests to confirm no regressions**

```
go test ./cmd/dotd/... -v
```

Expected: all pass.

- [ ] **Step 5: Update `.claude/docs/spec/cli.md`**

Replace the `### Top-level commands` table and subcommand sections to match the implementation. The updated spec should:

1. Remove `dotd link apply/check/remove` — links are managed by `dotd apply`/`dotd check`, not a separate `link` command.
2. Replace `dotd setup` with `dotd init` (the actual binary name is `init`).
3. Replace `dotd files list` with `dotd list` (no `files` subgroup).
4. Add `dotd env diff` to the env subcommands table.
5. Add `dotd package list` to the package subcommands table.
6. Note that `dotd compose apply` runs as part of `dotd apply` (no standalone subcommand).

Replace the section from `### Top-level commands` through `### Global flags` with:

```markdown
### Top-level commands

| Command | Description |
|---------|-------------|
| `dotd apply` | Full reconciliation — env → walk → filter → order → act → init.sh |
| `dotd check` | Validate all stages without making changes |
| `dotd init` | Interactive onboarding — scaffold dotfiles repo structure and config files |
| `dotd adopt <file>` | Import a file into the dotfiles repo _(not yet migrated to v2)_ |
| `dotd completion <shell>` | Generate shell completion script (bash, zsh, fish, powershell) |

### `dotd list` subcommands

| Command | Description |
|---------|-------------|
| `dotd list` | List active nodes (logical name, actions, path) |
| `dotd list --inactive` | List all nodes including inactive, with conditions |
| `dotd list --json` | Machine-readable JSON output |

### `dotd dag` subcommands

| Command | Description |
|---------|-------------|
| `dotd dag check` | Print active nodes in dependency order |

### `dotd env` subcommands

| Command | Description |
|---------|-------------|
| `dotd env show` | Display all resolved env key=value pairs |
| `dotd env get <key>` | Get a specific key |
| `dotd env set <key> <value>` | Set a key in `env.yaml` |
| `dotd env diff` | Show keys where `env.yaml` overrides shell-detected values |
| `dotd env edit` | Open `env.yaml` in `$EDITOR` |

### `dotd compose` subcommands

| Command | Description |
|---------|-------------|
| `dotd compose list` | List active compose targets |
| `dotd compose check` | Report stale or missing generated files |

Note: compose files are generated (and symlinked) by `dotd apply`.

### `dotd package` subcommands

| Command | Description |
|---------|-------------|
| `dotd package list` | List all packages declared in active nodes |
| `dotd package check` | Report package install status |
| `dotd package generate` | Generate a shell install script for active package requirements |

### `dotd config` subcommands

| Command | Description |
|---------|-------------|
| `dotd config show` | Show current config.yaml values |
| `dotd config get <key>` | Get a single config key |
| `dotd config set <key> <value>` | Set a key in config.yaml |
| `dotd config edit` | Open config.yaml in `$EDITOR` |
```

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/package.go cmd/dotd/compose_cmd.go cmd/dotd/dag_cmd.go .claude/docs/spec/cli.md
git commit -m "docs(spec): update cli.md to match v2 implementation; unhide package/compose/dag commands"
```

---

## Task 5: `internal/setup` tests

**Files:**
- Create: `internal/setup/setup_test.go`

`setup.Scaffold` (230 lines) has no tests. It creates three dirs (`shellrc/`, `conf/`, `bin/`), one root `.dagger` file, one `env.yaml`, and one `packages.yaml`. It is idempotent — calling it twice is safe.

- [ ] **Step 1: Write the failing tests**

Create `internal/setup/setup_test.go`:

```go
package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dot-dagger/internal/setup"
)

func TestScaffoldCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:  dir,
		EnvFilePath:  filepath.Join(dir, "env.yaml"),
		InitFilePath: filepath.Join(dir, "init.sh"),
	}

	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	for _, sub := range []string{"shellrc", "conf", "bin"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("expected dir %s to exist: %v", sub, err)
		}
	}

	created := countState(res, setup.KindDir, setup.StateCreated)
	if created != 3 {
		t.Errorf("expected 3 created dirs, got %d", created)
	}
}

func TestScaffoldCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:  dir,
		EnvFilePath:  filepath.Join(dir, "env.yaml"),
		InitFilePath: filepath.Join(dir, "init.sh"),
	}

	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	for _, name := range []string{"env.yaml", ".dagger", "packages.yaml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}

	created := countState(res, setup.KindFile, setup.StateCreated)
	if created != 3 {
		t.Errorf("expected 3 created files, got %d", created)
	}
}

func TestScaffoldIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:  dir,
		EnvFilePath:  filepath.Join(dir, "env.yaml"),
		InitFilePath: filepath.Join(dir, "init.sh"),
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("first Scaffold error = %v", err)
	}
	res, err := setup.Scaffold(opts)
	if err != nil {
		t.Fatalf("second Scaffold error = %v", err)
	}

	skipped := countState(res, setup.KindDir, setup.StateSkipped) +
		countState(res, setup.KindFile, setup.StateSkipped)
	if skipped != 6 {
		t.Errorf("expected 6 skipped actions on second run, got %d", skipped)
	}
}

func TestScaffoldWithSelectedManagers(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:      dir,
		EnvFilePath:      filepath.Join(dir, "env.yaml"),
		InitFilePath:     filepath.Join(dir, "init.sh"),
		SelectedManagers: []string{"dnf"},
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "packages.yaml"))
	if err != nil {
		t.Fatalf("read packages.yaml: %v", err)
	}
	if !contains(string(content), "dnf") {
		t.Errorf("packages.yaml should contain 'dnf', got: %s", content)
	}
}

func TestScaffoldWithDetectedValues(t *testing.T) {
	dir := t.TempDir()
	opts := setup.Options{
		DotfilesDir:    dir,
		EnvFilePath:    filepath.Join(dir, "env.yaml"),
		InitFilePath:   filepath.Join(dir, "init.sh"),
		DetectedOS:     "linux",
		DetectedDistro: "fedora",
		DetectedShell:  "zsh",
	}

	if _, err := setup.Scaffold(opts); err != nil {
		t.Fatalf("Scaffold error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "env.yaml"))
	if err != nil {
		t.Fatalf("read env.yaml: %v", err)
	}
	body := string(content)
	for _, want := range []string{"linux", "fedora", "zsh"} {
		if !contains(body, want) {
			t.Errorf("env.yaml should mention %q, got:\n%s", want, body)
		}
	}
}

func countState(res *setup.Result, kind setup.ActionKind, state setup.ActionState) int {
	n := 0
	for _, a := range res.Actions {
		if a.Kind == kind && a.State == state {
			n++
		}
	}
	return n
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./internal/setup/... -v
```

Expected: `cannot find package` or `no test files` — file doesn't exist yet. This step just confirms the file doesn't exist.

- [ ] **Step 3: Write the test file**

The code in Step 1 IS the implementation (it's test-only). Write `internal/setup/setup_test.go` with the content above.

- [ ] **Step 4: Run tests**

```
go test ./internal/setup/... -v
```

Expected: all 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/setup/setup_test.go
git commit -m "test(setup): add unit tests for Scaffold"
```

---

## Task 6: `internal/ecosystem` tests

**Files:**
- Create: `internal/ecosystem/ecosystem_test.go`

`ecosystem` (116 lines) has no tests. The most testable function is `ResolvePath` — it has real precedence logic. The `Default*` path functions test XDG overrides.

- [ ] **Step 1: Write the failing tests**

Create `internal/ecosystem/ecosystem_test.go`:

```go
package ecosystem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

func TestResolvePathCliArgWins(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "from-shell")
	got, err := ecosystem.ResolvePath("from-cli", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-cli" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-cli")
	}
}

func TestResolvePathShellVarBeatsFile(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "from-shell")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-shell" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-shell")
	}
}

func TestResolvePathFileBeatsDefault(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "from-file", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-file" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-file")
	}
}

func TestResolvePathUsesDefault(t *testing.T) {
	t.Setenv("DOTD_TEST_VAR", "")
	got, err := ecosystem.ResolvePath("", "DOTD_TEST_VAR", "", func() (string, error) {
		return "from-default", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-default" {
		t.Errorf("ResolvePath = %q, want %q", got, "from-default")
	}
}

func TestDefaultInitFileUsesXDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	got, err := ecosystem.DefaultInitFile()
	if err != nil {
		t.Fatalf("DefaultInitFile error = %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "init.sh")
	if got != want {
		t.Errorf("DefaultInitFile = %q, want %q", got, want)
	}
}

func TestDefaultEnvFileUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := ecosystem.DefaultEnvFile()
	if err != nil {
		t.Fatalf("DefaultEnvFile error = %v", err)
	}
	want := filepath.Join(tmp, "dot-dagger", "env.yaml")
	if got != want {
		t.Errorf("DefaultEnvFile = %q, want %q", got, want)
	}
}

func TestDefaultDotfilesFromEnvVar(t *testing.T) {
	t.Setenv("DOTFILES", "/my/dotfiles")
	got := ecosystem.DefaultDotfiles()
	if got != "/my/dotfiles" {
		t.Errorf("DefaultDotfiles = %q, want %q", got, "/my/dotfiles")
	}
}

func TestDefaultDotfilesFallsToCwd(t *testing.T) {
	t.Setenv("DOTFILES", "")
	wd, _ := os.Getwd()
	got := ecosystem.DefaultDotfiles()
	if got != wd {
		t.Errorf("DefaultDotfiles = %q, want cwd %q", got, wd)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./internal/ecosystem/... -v
```

Expected: `no test files` — file doesn't exist yet.

- [ ] **Step 3: Write the test file**

The code in Step 1 IS the test. Write `internal/ecosystem/ecosystem_test.go`.

- [ ] **Step 4: Run tests**

```
go test ./internal/ecosystem/... -v
```

Expected: all 8 tests pass.

- [ ] **Step 5: Run full test suite**

```
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ecosystem/ecosystem_test.go
git commit -m "test(ecosystem): add unit tests for ResolvePath and Default* functions"
```

---

## Self-Review

**Spec coverage:**
- `runApply`/`runCheck` dedup → Task 1 ✓
- `dotd env diff` → Task 2 ✓
- `dotd package list` → Task 3 ✓
- Unhide commands → Task 4 ✓
- Spec update → Task 4 ✓
- `internal/setup` tests → Task 5 ✓
- `internal/ecosystem` tests → Task 6 ✓
- `dotd adopt` migration → out of scope (noted in header)

**Placeholder scan:** No TBDs. All code blocks are complete.

**Type consistency:**
- `pipelineRun.result` is `*pipeline.ActResult` — matches `pipeline.Act` return type ✓
- `pipeline.RawNode` used in Walk/Filter/Order throughout ✓
- `setup.Result`, `setup.ActionKind`, `setup.ActionState` match `setup.go` definitions ✓
- `ecosystem.ResolvePath` signature `(cliArg, envVar, envFileVal string, defaultFn func() (string, error)) (string, error)` matches ✓
