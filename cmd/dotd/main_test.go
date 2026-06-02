package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// run executes the root command with the given args, capturing combined stdout+stderr.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// emptyDotfiles creates a minimal temp dotfiles directory (no files).
func emptyDotfiles(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// emptyEnvFile creates a temp env.yaml that is empty (flat v2 format).
func emptyEnvFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- dotd env show ---

func TestEnvShowEmpty(t *testing.T) {
	_, err := run(t, "env", "show",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
}

func TestEnvShowFileKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	// v2 env.yaml is a flat map[string]string
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "show",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
	if !strings.Contains(out, "context=work") {
		t.Errorf("output missing context=work: %q", out)
	}
}

func TestEnvShowEnvFlagOverride(t *testing.T) {
	out, err := run(t, "env", "show",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"--env", "os=testvalue",
	)
	if err != nil {
		t.Fatalf("env show error = %v", err)
	}
	if !strings.Contains(out, "os=testvalue") {
		t.Errorf("output missing overridden os=testvalue: %q", out)
	}
}

// --- dotd env get ---

func TestEnvGetExistingKey(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("mykey: myval\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "env", "get", "mykey",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get error = %v", err)
	}
	if strings.TrimSpace(out) != "myval" {
		t.Errorf("env get mykey = %q, want %q", strings.TrimSpace(out), "myval")
	}
}

func TestEnvGetMissingKey(t *testing.T) {
	_, err := run(t, "env", "get", "no-such-key",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

// --- dotd env set ---

func TestEnvSetWritesKey(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "env", "set", "context", "work",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env set error = %v", err)
	}

	out, err := run(t, "env", "get", "context",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env get after set error = %v", err)
	}
	if strings.TrimSpace(out) != "work" {
		t.Errorf("context = %q, want work", strings.TrimSpace(out))
	}
}

func TestEnvSetInvalidArgs(t *testing.T) {
	_, err := run(t, "env", "set",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for wrong number of args")
	}
}

// --- dotd completion ---

func TestCompletionBash(t *testing.T) {
	out, err := run(t, "completion", "bash")
	if err != nil {
		t.Fatalf("completion bash error = %v", err)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("expected bash completion output: %q", out[:min(len(out), 200)])
	}
}

func TestCompletionZsh(t *testing.T) {
	out, err := run(t, "completion", "zsh")
	if err != nil {
		t.Fatalf("completion zsh error = %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	_, err := run(t, "completion", "nosuchshell")
	if err == nil {
		t.Error("expected error for unsupported shell")
	}
}

// --- dotd check ---

func TestCheckEmptyRepo(t *testing.T) {
	// check on a fresh repo always reports init.sh missing — exit code non-zero is expected.
	// What we care about: it runs without panicking and produces symlink/init.sh summary.
	out, _ := run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if !strings.Contains(out, "symlinks:") {
		t.Errorf("expected symlinks summary in output: %q", out)
	}
}

func TestCheckAfterApply_Unit(t *testing.T) {
	dotfiles := t.TempDir()
	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	// Apply first so init.sh exists.
	_, err := run(t, "apply",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}

	// check should succeed since init.sh was created.
	_, err = run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("check after apply error = %v", err)
	}
}

// --- dotd apply --dry-run ---

func TestApplyDryRunEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output: %q", out)
	}
	// init.sh must not be created.
	if _, err := os.Stat(initFile); !os.IsNotExist(err) {
		t.Error("dry-run must not create init.sh")
	}
}

func TestApplyDryRunWithActiveNode(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// v2 annotation syntax: @when(os=linux), .dagger defaults source action
	daggerFile := filepath.Join(shellrcDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("# @when(os=linux)\nexport X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	// Dry-run reports the number of sourced nodes including the file name.
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output: %q", out)
	}
	// init.sh must not be created.
	if _, err := os.Stat(initFile); !os.IsNotExist(err) {
		t.Error("dry-run must not create init.sh")
	}
}

func TestApplyWritesInitSh(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	daggerFile := filepath.Join(shellrcDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	_, err := run(t, "apply",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}
	content, err := os.ReadFile(initFile)
	if err != nil {
		t.Fatalf("init.sh not created: %v", err)
	}
	if !strings.Contains(string(content), "base.sh") {
		t.Errorf("init.sh missing base.sh: %s", string(content))
	}
}

// --- dotd list ---

func TestListEmpty(t *testing.T) {
	_, err := run(t, "list",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
}

func TestListActiveNodes(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "shellrc.base") {
		t.Errorf("expected shellrc.base in list output: %q", out)
	}
	if strings.Contains(out, "shellrc.macos") {
		t.Errorf("shellrc.macos should be inactive and not shown: %q", out)
	}
}

func TestListInactiveFlag(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "macos.sh"), []byte("# @when(os=macos)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list", "--inactive",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"--env", "os=linux",
	)
	if err != nil {
		t.Fatalf("list --inactive error = %v", err)
	}
	if !strings.Contains(out, "shellrc.macos") {
		t.Errorf("expected shellrc.macos in --inactive output: %q", out)
	}
	if !strings.Contains(out, "inactive") {
		t.Errorf("expected 'inactive' tag in output: %q", out)
	}
}

// --- dotd bundle ---

func TestBundleNoFile(t *testing.T) {
	_, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
		"nonexistent.sh",
	)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBundleSimple(t *testing.T) {
	dotfiles := t.TempDir()
	shellrcDir := filepath.Join(dotfiles, "shellrc")
	if err := os.MkdirAll(shellrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, ecosystem.ConfigFile), []byte("defaults:\n  actions:\n    - source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrcDir, "base.sh"), []byte("export BASE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "bundle",
		"--env-file", emptyEnvFile(t),
		"--files", dotfiles,
		"shellrc/base.sh",
	)
	if err != nil {
		t.Fatalf("bundle error = %v", err)
	}
	if !strings.Contains(out, "export BASE=1") {
		t.Errorf("expected file content in bundle output: %q", out)
	}
}

// --- dotd env diff ---

func TestEnvDiffShowsOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("context: work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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

// --- dotd package list ---

func TestPackageListEmpty(t *testing.T) {
	dir := t.TempDir()
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

func TestPackageListShowsRequestAnnotation(t *testing.T) {
	dir := t.TempDir()
	shellrc := filepath.Join(dir, "shellrc")
	if err := os.MkdirAll(shellrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shellrc, "base.sh"), []byte("# @request(curl)\n"), 0o644); err != nil {
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
	if !strings.Contains(out, "curl") {
		t.Errorf("expected 'curl' in output, got %q", out)
	}
	if !strings.Contains(out, "request") {
		t.Errorf("expected 'request' in output, got %q", out)
	}
}

func TestResolveToFlag(t *testing.T) {
	tests := []struct {
		to   string
		name string
		want string
	}{
		{"config/dot-gitconfig-work", ".gitconfig", "config/dot-gitconfig-work"},
		{"config/", ".gitconfig", "config/.gitconfig"},
		{"bin/", "my-script", "bin/my-script"},
	}
	for _, tt := range tests {
		got := resolveToFlag(tt.to, tt.name)
		if got != tt.want {
			t.Errorf("resolveToFlag(%q, %q) = %q, want %q", tt.to, tt.name, got, tt.want)
		}
	}
}

// --- dotd setup ---

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

// --- dotd init ---

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

// --- dotd teardown ---

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

// --- dotd unapply ---

func TestUnapply_RemovesSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File with link destination annotation
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
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

// TestUnapply_PartialFailureExits1 verifies that when os.Remove fails on
// one of the planned targets, unapply exits non-zero and writes the failure
// message to stderr (AUDIT-033).
func TestUnapply_PartialFailureExits1(t *testing.T) {
	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, ecosystem.ConfigFile),
		[]byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n[core]\n  autocrlf = false\n"), 0o644); err != nil {
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

	// Sabotage the planned removal: strip write permission on the parent dir
	// so os.Remove on the symlink fails. We restore perms via t.Cleanup so
	// t.TempDir's own cleanup can run.
	if err := os.Chmod(home, 0o555); err != nil {
		t.Fatalf("chmod home: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })

	out, err := run(t, "unapply", "--yes",
		"--files", dotfiles, "--env-file", envFile,
		"--link-root", home, "--init-file", initFile,
	)
	if err == nil {
		t.Fatalf("unapply should have failed; output: %s", out)
	}
	if !strings.Contains(out, "removing") {
		t.Errorf("expected failure message containing 'removing', got: %s", out)
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
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
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
	confDir := filepath.Join(dotfiles, "config")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Set up .dagger file to mark files as links
	daggerFile := filepath.Join(confDir, ecosystem.ConfigFile)
	if err := os.WriteFile(daggerFile, []byte("link_root: \"~\"\ndefaults:\n  actions:\n    - link\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-gitconfig"),
		[]byte("# @link(~/.gitconfig)\n"), 0o644); err != nil {
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

// --- AUDIT-001: linkRoot default ---

// TestAdopt_DefaultLinkRoot verifies that adopt succeeds without --link-root by
// resolving $HOME as the default. Before the fix, cfg.linkRoot was empty and
// adopt would error with "act: HomeDir is required".
func TestAdopt_DefaultLinkRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Clear DOTD_LINK_ROOT so the test exercises the default-fn path (priority 4),
	// not the env-var path (priority 2).
	t.Setenv("DOTD_LINK_ROOT", "")

	dotfiles := t.TempDir()
	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A plain config file to adopt (hidden → conf/dot-testrc).
	src := filepath.Join(home, ".testrc")
	if err := os.WriteFile(src, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "adopt", "--yes",
		"--files", dotfiles,
		"--env-file", emptyEnvFile(t),
		src,
	)
	if err != nil && strings.Contains(err.Error(), "HomeDir is required") {
		t.Fatalf("adopt failed with HomeDir error — linkRoot default not set: %v", err)
	}
	// If adopt errors for another reason (e.g. missing .dagger conventions) that
	// is acceptable; what matters is that the HomeDir error does not appear.
}

// TestTeardown_UsesLinkRoot verifies that teardown resolves the shell RC file
// under cfg.linkRoot (set via --link-root), not under $HOME or cwd.
//
// Setup:
//   - linkRoot is a separate temp dir from $HOME — any RC lookup under $HOME
//     would find no file and leave the source line untouched.
//   - env.yaml contains shell: bash so DetectShellConfig is invoked and the
//     RC path is built as <linkRoot>/.bashrc.
//   - <linkRoot>/.bashrc contains the dot-dagger source line so teardown has
//     something to remove.
//
// Pass criterion: the source line is gone from <linkRoot>/.bashrc after teardown.
// Fail criterion: if the bug returns (linkRoot ignored → $HOME used instead),
// the RC under linkRoot would not be found, the source line would not be
// removed, and the assertion below would fail.
func TestTeardown_UsesLinkRoot(t *testing.T) {
	// $HOME is intentionally different from linkRoot so a mix-up is detectable.
	home := t.TempDir()
	t.Setenv("HOME", home)

	xdgData := t.TempDir()
	xdgConfig := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("DOTD_LINK_ROOT", "") // ensure env var doesn't override --link-root

	// linkRoot is the directory teardown must use when building the RC path.
	linkRoot := t.TempDir()

	// Compute the initFile path that resolvePaths will resolve:
	// DOTD_INIT_FILE is unset, so it falls through to DefaultInitFile →
	// $XDG_DATA_HOME/dot-dagger/init.sh.
	t.Setenv("DOTD_INIT_FILE", "") // force default-fn resolution
	initFile := filepath.Join(xdgData, "dot-dagger", "init.sh")

	// Create the XDG config dir and env.yaml with shell: bash.
	configDir := filepath.Join(xdgConfig, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(configDir, "env.yaml")
	if err := os.WriteFile(envPath, []byte("shell: bash\nos: linux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create <linkRoot>/.bashrc containing the dot-dagger source line.
	// DetectShellConfig(bash, linux, linkRoot) → linkRoot/.bashrc.
	// HasSourceLine and RemoveSourceLine both key on filepath.Base(initFile) = "init.sh".
	bashrc := filepath.Join(linkRoot, ".bashrc")
	sourceLine := `source "$HOME/.local/share/dot-dagger/init.sh"`
	bashrcContents := "# existing rc\n\n# dotd \xe2\x80\x94 generated shell init\n" + sourceLine + "\n"
	if err := os.WriteFile(bashrc, []byte(bashrcContents), 0o644); err != nil {
		t.Fatal(err)
	}

	// Confirm $HOME/.bashrc does NOT exist — so if teardown wrongly uses $HOME
	// it will silently skip removal rather than succeed.
	homeBashrc := filepath.Join(home, ".bashrc")

	_, err := run(t, "teardown", "--yes",
		"--link-root", linkRoot,
		"--init-file", initFile,
		"--files", emptyDotfiles(t),
		"--env-file", envPath,
	)
	if err != nil {
		t.Fatalf("teardown error = %v", err)
	}

	// The source line must be gone from <linkRoot>/.bashrc.
	got, readErr := os.ReadFile(bashrc)
	if readErr != nil {
		t.Fatalf("read %s: %v", bashrc, readErr)
	}
	if strings.Contains(string(got), "init.sh") {
		t.Errorf("source line still present in %s — teardown did not strip it:\n%s", bashrc, got)
	}

	// $HOME/.bashrc must not have been created — teardown must not have touched $HOME.
	if _, statErr := os.Stat(homeBashrc); statErr == nil {
		t.Errorf("teardown created %s — it looked up RC under $HOME instead of linkRoot", homeBashrc)
	}
}

// --- AUDIT-002: configPath overridable via --config / DOTD_CONFIG_FILE ---

// minimalCfg returns an appConfig pre-populated with an empty envFile so that
// resolvePaths can run without touching the real XDG config dir.
func minimalCfg(t *testing.T) *appConfig {
	t.Helper()
	return &appConfig{
		envFile: emptyEnvFile(t),
	}
}

// TestConfigPath_FlagOverride verifies that --config /tmp/x.yaml makes
// cfg.configPath resolve to that path after resolvePaths runs.
func TestConfigPath_FlagOverride(t *testing.T) {
	dir := t.TempDir()
	customConfig := filepath.Join(dir, "custom-config.yaml")
	// File need not exist — resolvePaths only resolves the path, not loads it
	// (Load handles the missing-file case gracefully).

	t.Setenv("DOTD_CONFIG_FILE", "") // ensure env var doesn't interfere

	cfg := minimalCfg(t)
	cfg.configPath = customConfig // simulates --config flag binding

	if err := resolvePaths(cfg); err != nil {
		t.Fatalf("resolvePaths error = %v", err)
	}

	if cfg.configPath != customConfig {
		t.Errorf("cfg.configPath = %q, want %q", cfg.configPath, customConfig)
	}
}

// --- AUDIT-003: setup/init honor --config override (cfg.configPath) ---

// TestSetup_HonorsConfigFlag verifies that "dotd setup --config /tmp/x.yaml"
// writes config.yaml to the override path and NOT to the XDG default.
// Before AUDIT-003 the fix, setup called dotcfg.DefaultPath() directly and
// silently ignored the --config override.
func TestSetup_HonorsConfigFlag(t *testing.T) {
	// Use a fresh XDG dir so the default path is somewhere we can detect.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", t.TempDir()) // valid dotfiles default
	t.Setenv("DOTD_CONFIG_FILE", "")  // ensure env var doesn't interfere

	// The override config path — must not be inside xdg so we can distinguish.
	overrideDir := t.TempDir()
	overrideConfig := filepath.Join(overrideDir, "my-config.yaml")

	// Simulate pressing Enter for all prompts (accept all defaults).
	input := strings.Repeat("\n", 10)
	root := newRootCmd()
	root.SetIn(strings.NewReader(input))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"setup", "--config", overrideConfig})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup --config error = %v\noutput:\n%s", err, buf.String())
	}

	// Override path must have been written.
	if _, err := os.Stat(overrideConfig); err != nil {
		t.Errorf("config.yaml not written to override path %s: %v", overrideConfig, err)
	}

	// Default XDG config must NOT exist (setup must not have written there).
	defaultConfig := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(defaultConfig); err == nil {
		t.Errorf("config.yaml written to default XDG path %s — --config override was ignored", defaultConfig)
	}
}

// TestInit_HonorsConfigFlag verifies that "dotd init --config /tmp/missing.yaml"
// errors with "no config found" when the override path does not exist.
// Before AUDIT-003, init called dotcfg.DefaultPath() and would check the XDG
// default instead of the override, masking the wrong path.
func TestInit_HonorsConfigFlag(t *testing.T) {
	// Ensure the XDG default config.yaml exists so that if init ignores the
	// --config override and checks the default, it would NOT error — letting us
	// detect the regression.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTD_CONFIG_FILE", "") // ensure env var doesn't interfere

	configDir := filepath.Join(xdg, "dot-dagger")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write the default config so it exists — init must still error because the
	// override path is missing.
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("dotfiles: /tmp/df\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override path that does NOT exist.
	missingConfig := filepath.Join(t.TempDir(), "no-such-config.yaml")

	_, err := run(t, "init",
		"--config", missingConfig,
		"--files", emptyDotfiles(t),
		"--env-file", emptyEnvFile(t),
	)
	if err == nil {
		t.Fatal("expected error when --config path is missing, got nil — --config override was ignored")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Errorf("expected 'no config found' error, got: %v", err)
	}
}

// TestConfigPath_EnvVarOverride verifies that DOTD_CONFIG_FILE=/tmp/x.yaml
// makes cfg.configPath resolve to that path after resolvePaths runs.
func TestConfigPath_EnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	customConfig := filepath.Join(dir, "env-config.yaml")

	t.Setenv("DOTD_CONFIG_FILE", customConfig)

	cfg := minimalCfg(t)
	// configPath is empty — DOTD_CONFIG_FILE should win.

	if err := resolvePaths(cfg); err != nil {
		t.Fatalf("resolvePaths error = %v", err)
	}

	if cfg.configPath != customConfig {
		t.Errorf("cfg.configPath = %q, want %q", cfg.configPath, customConfig)
	}
}
