# Lifecycle Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the monolithic `dotd init` with a clean three-tier lifecycle: `dotd setup` (system config), `dotd init` (directory scaffold), `dotd apply`/`unapply` (reconcile), `dotd teardown` (remove system config).

**Architecture:** `setup_cmd.go` and `teardown_cmd.go` own system-level config (config.yaml, env.yaml, shell RC). `init_cmd.go` is stripped to only scaffold `.dagger` files. `unapply_cmd.go` reverses `apply` by re-running the pipeline in dry-run mode and removing matching symlinks. `RemoveSourceLine` in `internal/setup/shell.go` is the new primitive teardown needs.

**Tech Stack:** Go, Cobra, `internal/config`, `internal/env`, `internal/pipeline`, `internal/setup`, `internal/ecosystem`

---

## File map

| File | Action | Purpose |
|------|--------|---------|
| `internal/setup/shell.go` | Modify | Add `RemoveSourceLine` |
| `internal/setup/shell_test.go` | Create | Tests for `RemoveSourceLine` |
| `cmd/dotd/init_cmd.go` | Rewrite | Strip to scaffold-only; add config.yaml precondition |
| `cmd/dotd/setup_cmd.go` | Create | Full interactive wizard: config.yaml + env.yaml |
| `cmd/dotd/teardown_cmd.go` | Create | Remove config.yaml, env.yaml, RC source line |
| `cmd/dotd/unapply_cmd.go` | Create | Remove symlinks by reversing the pipeline |
| `cmd/dotd/main.go` | Modify | Register setup, teardown, unapply |

---

## Task 1: `RemoveSourceLine` in `internal/setup/shell.go`

`AppendSourceLine` writes (verbatim):
```
\n# dotd — generated shell init\n<source line>\n
```

`RemoveSourceLine` scans line-by-line and removes the header + source line pair. Leaves surrounding content untouched.

**Files:**
- Modify: `internal/setup/shell.go`
- Create: `internal/setup/shell_test.go`

- [ ] **Step 1: Create `internal/setup/shell_test.go` with failing tests**

```go
package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/setup"
)

func writeTempRC(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "rc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func TestRemoveSourceLine_RemovesHeaderAndLine(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# existing content\n\n# dotd \xe2\x80\x94 generated shell init\nsource \"$HOME/.local/share/dot-dagger/init.sh\"\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if strings.Contains(string(got), "dotd") {
		t.Errorf("expected dotd lines removed, got:\n%s", string(got))
	}
	if !strings.Contains(string(got), "# existing content") {
		t.Errorf("expected existing content preserved, got:\n%s", string(got))
	}
}

func TestRemoveSourceLine_NoopWhenAbsent(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# just some rc content\nexport FOO=bar\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if string(got) != content {
		t.Errorf("expected unchanged content, got:\n%s", string(got))
	}
}

func TestRemoveSourceLine_NoopWhenFileMissing(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	if err := setup.RemoveSourceLine("/nonexistent/path/.bashrc", initFile); err != nil {
		t.Fatalf("expected nil error for missing RC file, got %v", err)
	}
}

func TestRemoveSourceLine_MultipleCallsIdempotent(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), "init.sh")
	content := "# header\n\n# dotd \xe2\x80\x94 generated shell init\nsource \"$HOME/.local/share/dot-dagger/init.sh\"\n# footer\n"
	rc := writeTempRC(t, content)

	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	if err := setup.RemoveSourceLine(rc, initFile); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(rc)
	if strings.Contains(string(got), "dotd") {
		t.Errorf("second call: dotd lines not removed:\n%s", string(got))
	}
	if !strings.Contains(string(got), "# footer") {
		t.Errorf("second call: footer removed:\n%s", string(got))
	}
}
```

Note: the `\xe2\x80\x94` sequence is the UTF-8 encoding of `—` (em dash). The header string in the test must match exactly what `AppendSourceLine` writes: `"# dotd — generated shell init"`.

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/rocne/git/dot-dagger
go test ./internal/setup/... -run TestRemoveSourceLine -v
```

Expected: compile error (`RemoveSourceLine` undefined).

- [ ] **Step 3: Add `RemoveSourceLine` to `internal/setup/shell.go`**

Add after `AppendSourceLine`:

```go
// RemoveSourceLine removes the dotd source line and its comment header from rcFile.
// The comment written by AppendSourceLine is "# dotd — generated shell init".
// No-op if rcFile does not exist or the lines are not present.
func RemoveSourceLine(rcFile, initFile string) error {
	data, err := os.ReadFile(rcFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("setup: read %s: %w", rcFile, err)
	}

	lines := strings.Split(string(data), "\n")
	needle := filepath.Base(initFile)
	const header = "# dotd \xe2\x80\x94 generated shell init"

	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if lines[i] == header {
			i++ // skip header
			// skip the following source line if it references our init file
			if i < len(lines) && strings.Contains(lines[i], "source") && strings.Contains(lines[i], needle) {
				i++
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}

	return os.WriteFile(rcFile, []byte(strings.Join(out, "\n")), 0o644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test ./internal/setup/... -run TestRemoveSourceLine -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/setup/shell.go internal/setup/shell_test.go
git commit -m "feat: add RemoveSourceLine to internal/setup/shell"
```

---

## Task 2: Rewrite `cmd/dotd/init_cmd.go`

Strip init to scaffold-only. Remove config/env/RC writing, `--yes` flag, and `maybeAddSourceLine`. Add a hard precondition check: config.yaml must exist.

**Files:**
- Rewrite: `cmd/dotd/init_cmd.go`
- Test: `cmd/dotd/main_test.go`

- [ ] **Step 1: Write failing test in `cmd/dotd/main_test.go`**

```go
func TestInit_RequiresConfig(t *testing.T) {
	// Fresh XDG dir so no config.yaml exists.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := run(t, "init",
		"--files", emptyDotfiles(t),
		"--env-file", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("expected error when config.yaml is absent, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./cmd/dotd/... -run TestInit_RequiresConfig -v
```

Expected: FAIL (current init doesn't check for config.yaml).

- [ ] **Step 3: Rewrite `cmd/dotd/init_cmd.go`**

Replace the entire file with:

```go
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold .dagger convention files in your dotfiles repo",
		Long: `Scaffold .dagger convention files in the configured dotfiles repo.

Prompts for shell scripts and config file directories.
Creates each directory if absent, writes .dagger if absent (idempotent).

Requires config.yaml — run 'dotd setup' first if you haven't already.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, cfg)
		},
	}
}

func runInit(cmd *cobra.Command, cfg *config) error {
	// Precondition: config.yaml must exist.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("no config found — run 'dotd setup' first")
	}

	reader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintln(cmd.OutOrStdout(), "Scaffold .dagger convention files — enter directory paths, empty to skip.")
	fmt.Fprintln(cmd.OutOrStdout())

	if err := scaffoldDaggerInteractive(reader, cmd, cfg.files); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Add dotfiles to your repo")
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: %s apply\n", ecosystem.ToolD)
	return nil
}

func scaffoldDaggerInteractive(reader *bufio.Reader, cmd *cobra.Command, dotfilesPath string) error {
	roles := []struct {
		name    string
		content string
	}{
		{"shell scripts directory (source action)", "defaults:\n  actions:\n    - source\n"},
		{"config files directory (link action)", "defaults:\n  actions:\n    - link\n"},
	}

	for _, role := range roles {
		dir, err := promptDefault(cmd.OutOrStdout(), reader, role.name, "", false)
		if err != nil {
			return err
		}
		if dir == "" {
			continue
		}
		dir = filepath.Join(dotfilesPath, filepath.Clean(dir))
		if err := scaffoldDagger(dir, role.content); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/.dagger\n", dir)
	}
	return nil
}

func scaffoldDagger(dir, content string) error {
	path := filepath.Join(dir, ".dagger")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — skip
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, defaultVal)
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

func expandTildeStr(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
```

- [ ] **Step 4: Run tests**

```
go test ./cmd/dotd/... -run TestInit -v
```

Expected: `TestInit_RequiresConfig` passes.

- [ ] **Step 5: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/init_cmd.go cmd/dotd/main_test.go
git commit -m "refactor: strip dotd init to scaffold-only, add config.yaml precondition"
```

---

## Task 3: Implement `cmd/dotd/setup_cmd.go`

`setup` is the interactive wizard that writes config.yaml and env.yaml. When config.yaml already exists, its values are shown as per-field defaults (user presses Enter to keep).

**Files:**
- Create: `cmd/dotd/setup_cmd.go`
- Modify: `cmd/dotd/main.go` (register command)
- Test: `cmd/dotd/main_test.go`

- [ ] **Step 1: Write failing tests in `cmd/dotd/main_test.go`**

```go
func TestSetup_WritesConfigAndEnv(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir()) // so default dotfiles path exists

	// Simulate pressing Enter for all prompts (accept defaults).
	input := strings.Repeat("\n", 10)

	root := newRootCmd()
	root.SetIn(strings.NewReader(input))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"setup"})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, buf.String())
	}

	configPath := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
	envPath := filepath.Join(xdg, "dot-dagger", "env.yaml")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("env.yaml not written: %v", err)
	}
}

func TestSetup_UsesCurrentValuesAsDefaults(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	// Pre-write config.yaml with a known dotfiles path.
	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingDotfiles := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("dotfiles: "+existingDotfiles+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Accept all defaults (Enter for each prompt).
	input := strings.Repeat("\n", 10)
	root := newRootCmd()
	root.SetIn(strings.NewReader(input))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"setup"})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, buf.String())
	}

	content, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), existingDotfiles) {
		t.Errorf("existing dotfiles path not preserved, got:\n%s", string(content))
	}
}

func TestSetup_SkipsEnvIfExists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir())

	// Pre-write env.yaml.
	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envContent := "context: work\n"
	if err := os.WriteFile(filepath.Join(configDir, "env.yaml"), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	input := strings.Repeat("\n", 10)
	root := newRootCmd()
	root.SetIn(strings.NewReader(input))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"setup"})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, buf.String())
	}

	got, _ := os.ReadFile(filepath.Join(configDir, "env.yaml"))
	if string(got) != envContent {
		t.Errorf("env.yaml modified: got %q, want %q", string(got), envContent)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./cmd/dotd/... -run TestSetup -v
```

Expected: FAIL (`setup` not registered).

- [ ] **Step 3: Create `cmd/dotd/setup_cmd.go`**

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newSetupCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard: configure dot-dagger at the system level",
		Long: `Configure dot-dagger for this machine.

Writes config.yaml and (if absent) env.yaml to the platform config dir.
If config.yaml already exists, current values are shown as defaults.

Does not create symlinks or scaffold .dagger files.
Run 'dotd init' next to scaffold your dotfiles repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, cfg)
		},
	}
}

func runSetup(cmd *cobra.Command, cfg *config) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Load existing config.yaml — returns empty Config (no error) if absent.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	existing, err := dotcfg.Load(configPath)
	if err != nil {
		return fmt.Errorf("setup: load existing config: %w", err)
	}

	isUpdate := existing.Dotfiles != "" || existing.BinDir != "" || existing.GeneratedDir != "" || existing.LinkRoot != ""
	if isUpdate {
		fmt.Fprintln(cmd.OutOrStdout(), "Updating dot-dagger configuration — press Enter to keep current value.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "dot-dagger setup wizard — press Enter to accept defaults.")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Dotfiles path. Use cfg.files — already resolved by resolvePaths (DOTD_FILES → DOTFILES → config → default).
	dotfilesDefault := existing.Dotfiles
	if dotfilesDefault == "" {
		dotfilesDefault = cfg.files
	}
	dotfilesPath, err := promptDefault(cmd.OutOrStdout(), reader, "Dotfiles repo (your dotfiles git repository)", dotfilesDefault, false)
	if err != nil {
		return err
	}
	dotfilesPath = expandTildeStr(dotfilesPath, home)
	dotfilesPath, err = filepath.Abs(dotfilesPath)
	if err != nil {
		return err
	}

	// Bin dir. Use cfg.binDir — already resolved by resolvePaths.
	binDirDefault := existing.BinDir
	if binDirDefault == "" {
		binDirDefault = cfg.binDir
	}
	binDir, err := promptDefault(cmd.OutOrStdout(), reader, "Bin directory (generated shell wrappers)", binDirDefault, false)
	if err != nil {
		return err
	}
	binDir = expandTildeStr(binDir, home)
	binDir, err = filepath.Abs(binDir)
	if err != nil {
		return err
	}

	// Generated dir. Use cfg.generatedDir — already resolved by resolvePaths.
	generatedDirDefault := existing.GeneratedDir
	if generatedDirDefault == "" {
		generatedDirDefault = cfg.generatedDir
	}
	generatedDir, err := promptDefault(cmd.OutOrStdout(), reader, "Generated files directory (assembled compose targets)", generatedDirDefault, false)
	if err != nil {
		return err
	}
	generatedDir = expandTildeStr(generatedDir, home)
	generatedDir, err = filepath.Abs(generatedDir)
	if err != nil {
		return err
	}

	// Link root. Use cfg.linkRoot — already resolved by resolvePaths.
	linkRootDefault := existing.LinkRoot
	if linkRootDefault == "" {
		linkRootDefault = cfg.linkRoot
	}
	linkRoot, err := promptDefault(cmd.OutOrStdout(), reader, "Symlink root (where dotfiles symlink to, usually $HOME)", linkRootDefault, false)
	if err != nil {
		return err
	}
	linkRoot = expandTildeStr(linkRoot, home)
	linkRoot, err = filepath.Abs(linkRoot)
	if err != nil {
		return err
	}

	// Write config.yaml.
	toolCfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		LinkRoot:     linkRoot,
	}
	if err := dotcfg.Save(configPath, toolCfg); err != nil {
		return fmt.Errorf("setup: save config.yaml: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)

	// Write env.yaml only if absent.
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(envPath); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
			return fmt.Errorf("setup: mkdir %s: %w", filepath.Dir(envPath), err)
		}
		envContent := fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)
		if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
			return fmt.Errorf("setup: write env.yaml: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", envPath)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "exists %s (not modified)\n", envPath)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Run: %s init  (scaffold .dagger convention files)\n", ecosystem.ToolD)
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Add dotfiles to your repo")
	fmt.Fprintf(cmd.OutOrStdout(), "  3. Run: %s apply\n", ecosystem.ToolD)
	return nil
}
```

- [ ] **Step 4: Register `setup` in `cmd/dotd/main.go`**

In `newRootCmd`, add `newSetupCmd(cfg)` to `root.AddCommand(...)`:

```go
root.AddCommand(
    getOSCmd,
    getHostnameCmd,
    newConfigCmd(),
    newSetupCmd(cfg),   // ← add this line
    newInitCmd(cfg),
    newAdoptCmd(cfg),
    newApplyCmd(cfg),
    // ... rest unchanged
)
```

- [ ] **Step 5: Run tests**

```
go test ./cmd/dotd/... -run TestSetup -v
```

Expected: all 3 setup tests pass.

- [ ] **Step 6: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/dotd/setup_cmd.go cmd/dotd/main.go cmd/dotd/main_test.go
git commit -m "feat: add dotd setup command — interactive config wizard"
```

---

## Task 4: Implement `cmd/dotd/teardown_cmd.go`

Teardown removes config.yaml, env.yaml, and the dotd RC source line. Shows a preview, prompts `[y/N]`, supports `--yes`. Warns (non-fatal) if active symlinks or `.dagger` files detected. Validates before touching anything.

**Files:**
- Create: `cmd/dotd/teardown_cmd.go`
- Modify: `cmd/dotd/main.go` (register command)
- Test: `cmd/dotd/main_test.go`

- [ ] **Step 1: Write failing tests in `cmd/dotd/main_test.go`**

```go
func TestTeardown_RemovesConfigAndEnv(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	envPath := filepath.Join(configDir, "env.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--env-file", envPath,
	)
	if err != nil {
		t.Fatalf("teardown error = %v", err)
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config.yaml should be removed")
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error("env.yaml should be removed")
	}
}

func TestTeardown_SkipsAbsentFiles(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	out, err := run(t, "teardown", "--yes",
		"--files", emptyDotfiles(t),
		"--env-file", filepath.Join(xdg, "env.yaml"), // doesn't exist
	)
	if err != nil {
		t.Fatalf("teardown error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "not found") && !strings.Contains(out, "skip") {
		t.Errorf("expected 'not found'/'skip' in output: %q", out)
	}
}

func TestTeardown_CancelExits0(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetIn(strings.NewReader("n\n"))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"teardown",
		"--files", emptyDotfiles(t),
		"--env-file", filepath.Join(xdg, "env.yaml"),
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("teardown cancel should exit 0, got %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Error("config.yaml should be preserved on cancel")
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", buf.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./cmd/dotd/... -run TestTeardown -v
```

Expected: FAIL (`teardown` not registered).

- [ ] **Step 3: Create `cmd/dotd/teardown_cmd.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/setup"
	"github.com/spf13/cobra"
)

func newTeardownCmd(cfg *config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "teardown",
		Short: "Remove dot-dagger system config (config.yaml, env.yaml, RC source line)",
		Long: `Remove dot-dagger system-level configuration from this machine.

Removes:
  - config.yaml from the platform config dir
  - env.yaml from the platform config dir
  - The dotd source line from the shell RC file (if detected)

Does NOT remove symlinks or .dagger files.
Run 'dotd unapply' first to remove symlinks, then 'dotd teardown'.

Shows a preview and prompts for confirmation before making any changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeardown(cmd, cfg, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runTeardown(cmd *cobra.Command, cfg *config, yes bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	// Pre-action check: warn if active symlinks detected.
	// Non-fatal if walk fails (env.yaml or dotfiles repo may be absent).
	if prun, err := runPipeline(cfg, true); err == nil {
		if len(prun.result.Links) > 0 {
			fmt.Fprintf(out, "warning: %d symlink(s) still active — consider running 'dotd unapply' first\n", len(prun.result.Links))
		}
	}

	// Pre-action check: warn if .dagger files still present.
	if cfg.files != "" && hasDaggerFiles(cfg.files) {
		fmt.Fprintln(out, "warning: .dagger files present in dotfiles repo — these will not be removed")
	}

	// Determine paths. Both call DefaultPath() directly — teardown removes the
	// system-level files regardless of any --env-file / --files flag overrides.
	// This is a deliberate exception to the canonical resolution rule.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}

	// Determine RC file path — requires env.yaml to know the shell.
	// If resolveEnv fails (env.yaml absent etc.), RC stripping is skipped.
	rcFile := ""
	if resolved, rerr := resolveEnv(cfg); rerr == nil {
		if shell := resolved["shell"]; shell != "" {
			osName := resolved["os"]
			if sc, ok, _ := setup.DetectShellConfig(shell, osName); ok {
				has, _ := setup.HasSourceLine(sc.RCFile, cfg.initFile)
				if has {
					rcFile = sc.RCFile
				}
			}
		}
	}

	// Stat each file to determine what exists.
	configExists := fileExists(configPath)
	envExists := fileExists(envPath)

	// Preview.
	fmt.Fprintln(out, "\nWill remove:")
	if configExists {
		fmt.Fprintf(out, "  %s\n", configPath)
	} else {
		fmt.Fprintf(out, "  %s (not found, skip)\n", configPath)
	}
	if envExists {
		fmt.Fprintf(out, "  %s\n", envPath)
	} else {
		fmt.Fprintf(out, "  %s (not found, skip)\n", envPath)
	}
	if rcFile != "" {
		fmt.Fprintf(out, "  source line from %s\n", rcFile)
	} else {
		fmt.Fprintln(out, "  RC source line (not detected, skip)")
	}

	// Confirmation.
	if !yes {
		fmt.Fprint(out, "\nProceed? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "cancelled")
			return nil
		}
	}

	// Execute.
	if configExists {
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("teardown: remove %s: %w", configPath, err)
		}
		fmt.Fprintf(out, "removed %s\n", configPath)
	} else {
		fmt.Fprintf(out, "not found, skip: %s\n", configPath)
	}

	if envExists {
		if err := os.Remove(envPath); err != nil {
			return fmt.Errorf("teardown: remove %s: %w", envPath, err)
		}
		fmt.Fprintf(out, "removed %s\n", envPath)
	} else {
		fmt.Fprintf(out, "not found, skip: %s\n", envPath)
	}

	if rcFile != "" {
		if err := setup.RemoveSourceLine(rcFile, cfg.initFile); err != nil {
			return fmt.Errorf("teardown: strip RC source line: %w", err)
		}
		fmt.Fprintf(out, "removed source line from %s\n", rcFile)
	}

	// Prune config dir if now empty.
	configDir := filepath.Dir(configPath)
	if entries, err := os.ReadDir(configDir); err == nil && len(entries) == 0 {
		if err := os.Remove(configDir); err == nil {
			fmt.Fprintf(out, "removed %s (empty)\n", configDir)
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasDaggerFiles reports whether any .dagger file exists under root.
func hasDaggerFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == ".dagger" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
```

- [ ] **Step 4: Register `teardown` in `cmd/dotd/main.go`**

```go
root.AddCommand(
    // ... existing
    newSetupCmd(cfg),
    newInitCmd(cfg),
    newTeardownCmd(cfg),  // ← add this
    // ... existing
)
```

- [ ] **Step 5: Run tests**

```
go test ./cmd/dotd/... -run TestTeardown -v
```

Expected: all 3 teardown tests pass.

- [ ] **Step 6: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/dotd/teardown_cmd.go cmd/dotd/main.go cmd/dotd/main_test.go
git commit -m "feat: add dotd teardown command"
```

---

## Task 5: Implement `cmd/dotd/unapply_cmd.go`

Unapply re-runs the pipeline dry-run to get the link plan, then removes symlinks where `readlink(Dest) == Src`. `--all` skips predicate filtering and removes any symlink pointing into the dotfiles repo.

**Before writing code, run:**
```
grep -n "LinkResult\|Links\b" /home/rocne/git/dot-dagger/internal/pipeline/act.go | head -20
```
This confirms the exported type name for the link results slice. The `runPipeline` function already exposes `run.result.Links` where each element has `.Src` and `.Dest`.

**Files:**
- Create: `cmd/dotd/unapply_cmd.go`
- Modify: `cmd/dotd/main.go` (register command)
- Test: `cmd/dotd/main_test.go`

- [ ] **Step 1: Write failing tests in `cmd/dotd/main_test.go`**

```go
func TestUnapply_RemovesSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @action link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	initFile := filepath.Join(t.TempDir(), "init.sh")
	envFile := emptyEnvFile(t)

	// Apply first.
	if _, err := run(t, "apply",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	linkDest := filepath.Join(home, ".gitconfig")
	if _, err := os.Lstat(linkDest); err != nil {
		t.Fatalf("symlink not created by apply: %v", err)
	}

	// Unapply.
	if _, err := run(t, "unapply", "--yes",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	); err != nil {
		t.Fatalf("unapply error = %v", err)
	}

	if _, err := os.Lstat(linkDest); !os.IsNotExist(err) {
		t.Error("symlink should be removed after unapply")
	}
}

func TestUnapply_NothingToRemove(t *testing.T) {
	home := t.TempDir()
	out, err := run(t, "unapply", "--yes",
		"--files", emptyDotfiles(t),
		"--env-file", emptyEnvFile(t),
		"--link-root", home,
		"--init-file", filepath.Join(t.TempDir(), "init.sh"),
	)
	if err != nil {
		t.Fatalf("unapply error = %v", err)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("expected 'nothing to remove', got %q", out)
	}
}

func TestUnapply_DryRunPreservesSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @action link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	initFile := filepath.Join(t.TempDir(), "init.sh")
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	out, err := run(t, "unapply", "--dry-run", "--yes",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("unapply --dry-run error = %v", err)
	}

	// Symlink must still exist.
	if _, err := os.Lstat(filepath.Join(home, ".gitconfig")); err != nil {
		t.Error("dry-run must not remove symlink")
	}
	if !strings.Contains(out, ".gitconfig") {
		t.Errorf("dry-run should mention .gitconfig in preview: %q", out)
	}
}

func TestUnapply_CancelExits0(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @action link(~/.gitconfig)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	initFile := filepath.Join(t.TempDir(), "init.sh")
	envFile := emptyEnvFile(t)

	if _, err := run(t, "apply",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	root := newRootCmd()
	root.SetIn(strings.NewReader("n\n"))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"unapply",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unapply cancel should exit 0, got %v", err)
	}

	if _, err := os.Lstat(filepath.Join(home, ".gitconfig")); err != nil {
		t.Error("symlink must be preserved on cancel")
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", buf.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./cmd/dotd/... -run TestUnapply -v
```

Expected: FAIL (`unapply` not registered).

- [ ] **Step 3: Check the `pipeline.ActResult.Links` type**

```
grep -n "Links\|LinkResult\|type.*Result" /home/rocne/git/dot-dagger/internal/pipeline/act.go | head -20
```

Note the exact type. The existing `runApply` code uses `run.result.Links` with `.Src` and `.Dest` fields. Match whatever types are already used there.

- [ ] **Step 4: Create `cmd/dotd/unapply_cmd.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newUnapplyCmd(cfg *config) *cobra.Command {
	var yes bool
	var all bool
	cmd := &cobra.Command{
		Use:   "unapply",
		Short: "Remove symlinks created by 'dotd apply'",
		Long: `Remove symlinks that were created by 'dotd apply'.

Re-runs the pipeline to determine the expected link plan, then removes each
symlink whose destination points to the expected source file.

Use --all to remove all symlinks pointing into the dotfiles repo regardless
of @when predicates — useful when you applied with --env flags.

Examples:
  dotd unapply
  dotd unapply --dry-run
  dotd unapply --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnapply(cmd, cfg, yes, all)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&all, "all", false, "remove all dotfiles symlinks regardless of @when predicates")
	return cmd
}

func runUnapply(cmd *cobra.Command, cfg *config, yes, all bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	// Collect the link plan.
	type linkPair struct{ src, dest string }
	var planned []linkPair

	if all {
		// Walk all nodes (no predicate filter), get all link destinations.
		nodes, _, err := pipeline.Walk(cfg.files)
		if err != nil {
			return fmt.Errorf("walk %s: %w", cfg.files, err)
		}
		ordered, err := pipeline.Order(nodes)
		if err != nil {
			return fmt.Errorf("order: %w", err)
		}
		res, err := pipeline.Act(ordered, buildActOptions(cfg, true))
		if err != nil {
			return fmt.Errorf("act: %w", err)
		}
		for _, lnk := range res.Links {
			planned = append(planned, linkPair{src: lnk.Src, dest: lnk.Dest})
		}
	} else {
		prun, err := runPipeline(cfg, true)
		if err != nil {
			return err
		}
		for _, lnk := range prun.result.Links {
			planned = append(planned, linkPair{src: lnk.Src, dest: lnk.Dest})
		}
	}

	// Determine which planned links are currently active symlinks.
	dotfilesRoot := cfg.files + string(filepath.Separator)
	var toRemove []string
	for _, lnk := range planned {
		target, err := os.Readlink(lnk.dest)
		if err != nil {
			continue // not a symlink or missing
		}
		if all {
			// Remove if symlink points into dotfiles repo.
			if strings.HasPrefix(target, dotfilesRoot) || target == cfg.files {
				toRemove = append(toRemove, lnk.dest)
			}
		} else {
			// Remove only if target matches exactly.
			if target == lnk.src {
				toRemove = append(toRemove, lnk.dest)
			}
		}
	}

	// Check for init.sh.
	initShExists := fileExists(cfg.initFile)

	if len(toRemove) == 0 && !initShExists {
		fmt.Fprintln(out, "nothing to remove")
		return nil
	}

	// Preview.
	if initShExists {
		fmt.Fprintf(out, "Will remove %d symlink(s) and init.sh:\n", len(toRemove))
	} else {
		fmt.Fprintf(out, "Will remove %d symlink(s):\n", len(toRemove))
	}
	for _, dest := range toRemove {
		fmt.Fprintf(out, "  %s\n", dest)
	}
	if initShExists {
		fmt.Fprintf(out, "  %s\n", cfg.initFile)
	}

	// Dry-run stops here.
	if cfg.dryRun {
		return nil
	}

	// Confirmation.
	if !yes {
		fmt.Fprint(out, "\nProceed? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "cancelled")
			return nil
		}
	}

	// Execute.
	for _, dest := range toRemove {
		if err := os.Remove(dest); err != nil {
			fmt.Fprintf(out, "error removing %s: %v\n", dest, err)
			continue
		}
		fmt.Fprintf(out, "removed %s\n", dest)
	}
	if initShExists {
		if err := os.Remove(cfg.initFile); err != nil {
			fmt.Fprintf(out, "error removing %s: %v\n", cfg.initFile, err)
		} else {
			fmt.Fprintf(out, "removed %s\n", cfg.initFile)
		}
	}

	return nil
}
```

- [ ] **Step 5: Register `unapply` in `cmd/dotd/main.go`**

```go
root.AddCommand(
    // ... existing
    newApplyCmd(cfg),
    newUnapplyCmd(cfg),  // ← add after apply
    // ... existing
)
```

- [ ] **Step 6: Build to catch type errors**

```
go build ./cmd/dotd/...
```

Fix any type errors (e.g., if `pipeline.Act` signature differs from what's written above).

- [ ] **Step 7: Run tests**

```
go test ./cmd/dotd/... -run TestUnapply -v
```

Expected: all 4 tests pass.

- [ ] **Step 8: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 9: Commit**

```bash
git add cmd/dotd/unapply_cmd.go cmd/dotd/main.go cmd/dotd/main_test.go
git commit -m "feat: add dotd unapply command"
```

---

## Task 6: Smoke test and push

- [ ] **Step 1: Verify `dotd --help` shows all new commands**

```
go run ./cmd/dotd --help
```

Expected: `setup`, `init`, `teardown`, `unapply` all appear.

- [ ] **Step 2: Run full test suite**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 3: Check PR is still open, then push**

```bash
gh pr view
git push
```

---

## Self-review checklist

- [x] R1 (setup): wizard with current-values-as-defaults when config.yaml exists
- [x] R1: writes config.yaml always, env.yaml only if absent
- [x] R1: does NOT touch RC file (WIP — deferred)
- [x] R2 (init): hard precondition check for config.yaml → "run dotd setup first"
- [x] R2: scaffold-only — no config/env/RC writing
- [x] R2: idempotent (.dagger skipped if exists)
- [x] R2: one prompt per role, empty = skip
- [x] R3 (teardown): pre-check warns on active symlinks (non-fatal if walk fails)
- [x] R3: warns on .dagger files (non-fatal)
- [x] R3: fail-fast atomicity — config.yaml removed first; error before touching env.yaml or RC
- [x] R3: preview + [y/N] + --yes
- [x] R3: RC strip skipped if resolveEnv fails (env.yaml absent → rcFile stays "")
- [x] R3: "not found, skip" for absent files
- [x] R3: prunes config dir if empty after removal
- [x] R4 (unapply): re-runs pipeline dry-run for filtered link plan (default mode)
- [x] R4: --all uses all nodes without predicate filter
- [x] R4: removes only exact-match symlinks (default) or into-dotfiles-repo symlinks (--all)
- [x] R4: removes init.sh if present
- [x] R4: preview + [y/N] + --yes
- [x] R4: --dry-run respects global cfg.dryRun flag
- [x] R4: "nothing to remove" exits 0
- [x] R4: "cancelled" exits 0
