package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Empty(t *testing.T) {
	dir := t.TempDir()
	m, err := Load(filepath.Join(dir, "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("want empty map, got %v", m)
	}
}

func TestLoad_StaticValues(t *testing.T) {
	dir := t.TempDir()
	content := "os: linux\ncontext: work\nhostname: mybox\n"
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m["os"] != "linux" || m["context"] != "work" || m["hostname"] != "mybox" {
		t.Errorf("got %v", m)
	}
}

func TestLoad_ShellExpressionRaw(t *testing.T) {
	dir := t.TempDir()
	content := "os: $(uname -s)\ncontext: work\n"
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m["os"] != "$(uname -s)" {
		t.Errorf("Load should return raw unexpanded value, got %q", m["os"])
	}
}

func TestExpand_StaticPassThrough(t *testing.T) {
	raw := map[string]string{"context": "work", "os": "linux"}
	got, err := Expand(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["context"] != "work" || got["os"] != "linux" {
		t.Errorf("got %v", got)
	}
}

func TestExpand_ShellExpression(t *testing.T) {
	raw := map[string]string{"greeting": "$(echo hello)"}
	got, err := Expand(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["greeting"] != "hello" {
		t.Errorf("Expand greeting = %q, want %q", got["greeting"], "hello")
	}
}

func TestExpand_FailedCommand_EmptyString(t *testing.T) {
	raw := map[string]string{"bad": "$(exit 1)"}
	got, err := Expand(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["bad"] != "" {
		t.Errorf("failed command should produce empty string, got %q", got["bad"])
	}
}

func TestExpand_NotAnExpression(t *testing.T) {
	raw := map[string]string{"key": "$(incomplete"}
	got, err := Expand(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["key"] != "$(incomplete" {
		t.Errorf("got %q", got["key"])
	}
}

func TestResolve_Precedence(t *testing.T) {
	expanded := map[string]string{"os": "linux", "context": "personal"}
	shellVars := map[string]string{"context": "work"}
	cliFlags := map[string]string{"os": "macos"}

	got := Resolve(cliFlags, shellVars, expanded)

	if got["os"] != "macos" {
		t.Errorf("cliFlags should win for os, got %q", got["os"])
	}
	if got["context"] != "work" {
		t.Errorf("shellVars should win over expanded for context, got %q", got["context"])
	}
}

func TestResolve_MergesAll(t *testing.T) {
	expanded := map[string]string{"hostname": "mybox"}
	shellVars := map[string]string{"context": "work"}
	cliFlags := map[string]string{}

	got := Resolve(cliFlags, shellVars, expanded)

	if got["hostname"] != "mybox" || got["context"] != "work" {
		t.Errorf("got %v", got)
	}
}

// TestResolve_TripleOverride exercises all three layers overriding the same key
// — cli wins per the precedence contract.
func TestResolve_TripleOverride(t *testing.T) {
	expanded := map[string]string{"os": "expanded"}
	shellVars := map[string]string{"os": "shell"}
	cliFlags := map[string]string{"os": "cli"}

	got := Resolve(cliFlags, shellVars, expanded)
	if got["os"] != "cli" {
		t.Errorf("cli should win triple-override, got %q", got["os"])
	}
}

// TestSave_UnwritableDirErrors verifies that Save wraps the underlying
// filesystem error so callers can detect "couldn't write env file".
func TestSave_UnwritableDirErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "subdir", "env.yaml")
	err := Save(path, map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected Save to fail on unwritable dir")
	}
}

// TestLoad_MalformedYAMLErrors verifies that load returns a wrapped error
// when the input isn't valid YAML.
func TestLoad_MalformedYAMLErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte("not valid: :yaml:\n  - bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected malformed YAML to error")
	}
}

// TestResolve_DoesNotMutateInputs verifies that Resolve treats its arguments
// as read-only — callers can pass shared maps without fearing aliasing bugs.
func TestResolve_DoesNotMutateInputs(t *testing.T) {
	expanded := map[string]string{"a": "e"}
	shellVars := map[string]string{"b": "s"}
	cliFlags := map[string]string{"c": "c"}

	expandedSnap := map[string]string{"a": "e"}
	shellVarsSnap := map[string]string{"b": "s"}
	cliFlagsSnap := map[string]string{"c": "c"}

	_ = Resolve(cliFlags, shellVars, expanded)

	for k, v := range expandedSnap {
		if expanded[k] != v {
			t.Errorf("expanded mutated: key %q now %q, was %q", k, expanded[k], v)
		}
	}
	if len(expanded) != len(expandedSnap) {
		t.Errorf("expanded grew/shrunk: %d entries, want %d", len(expanded), len(expandedSnap))
	}
	for k, v := range shellVarsSnap {
		if shellVars[k] != v {
			t.Errorf("shellVars mutated: key %q now %q, was %q", k, shellVars[k], v)
		}
	}
	for k, v := range cliFlagsSnap {
		if cliFlags[k] != v {
			t.Errorf("cliFlags mutated: key %q now %q, was %q", k, cliFlags[k], v)
		}
	}
}

func TestShellVars_ExtractsDOTD(t *testing.T) {
	environ := []string{
		"DOTD_CONTEXT=work",
		"DOTD_OS=macos",
		"HOME=/home/user",
		"DOTD_=ignored",
	}
	got := ShellVars(environ)
	if got["context"] != "work" {
		t.Errorf("context = %q", got["context"])
	}
	if got["os"] != "macos" {
		t.Errorf("os = %q", got["os"])
	}
	if _, ok := got["home"]; ok {
		t.Error("HOME should not be extracted")
	}
	if _, ok := got[""]; ok {
		t.Error("DOTD_ with empty suffix should be ignored")
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	raw := map[string]string{"os": "$(dotd get-os)", "context": "work"}
	if err := Save(path, raw); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["os"] != "$(dotd get-os)" || got["context"] != "work" {
		t.Errorf("round-trip failed: got %v", got)
	}
}
