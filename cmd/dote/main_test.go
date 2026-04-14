package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func envFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestShowBuiltins(t *testing.T) {
	out, err := run(t, "show", "--env-file", envFile(t, "env: {}\n"))
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	for _, key := range []string{"os=", "shell="} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing %s: %q", key, out)
		}
	}
}

func TestShowFileKeys(t *testing.T) {
	out, err := run(t, "show", "--env-file", envFile(t, "env:\n  context: work\n"))
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	if !strings.Contains(out, "context=work") {
		t.Errorf("output missing context=work: %q", out)
	}
}

func TestShowEnvFlagOverride(t *testing.T) {
	out, err := run(t, "show",
		"--env-file", envFile(t, "env: {}\n"),
		"--env", "os=testvalue",
	)
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	if !strings.Contains(out, "os=testvalue") {
		t.Errorf("output missing os=testvalue: %q", out)
	}
}

func TestShowEnvFlagOverridesFileKey(t *testing.T) {
	out, err := run(t, "show",
		"--env-file", envFile(t, "env:\n  context: work\n"),
		"--env", "context=personal",
	)
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	if !strings.Contains(out, "context=personal") {
		t.Errorf("--env flag should override file key: %q", out)
	}
	if strings.Contains(out, "context=work") {
		t.Errorf("file value should be overridden: %q", out)
	}
}

func TestShowMissingEnvFile(t *testing.T) {
	// Missing file = no error, just builtins.
	out, err := run(t, "show", "--env-file", "/nonexistent/env.yaml")
	if err != nil {
		t.Fatalf("show error = %v for missing env file", err)
	}
	if !strings.Contains(out, "os=") {
		t.Errorf("expected builtins even with missing env file: %q", out)
	}
}

func TestShowOutputSorted(t *testing.T) {
	out, err := run(t, "show",
		"--env-file", envFile(t, "env:\n  zebra: z\n  apple: a\n"),
	)
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Find apple and zebra positions.
	appleIdx, zebraIdx := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, "apple=") {
			appleIdx = i
		}
		if strings.HasPrefix(l, "zebra=") {
			zebraIdx = i
		}
	}
	if appleIdx < 0 || zebraIdx < 0 {
		t.Fatalf("apple or zebra missing from output: %q", out)
	}
	if appleIdx > zebraIdx {
		t.Errorf("output not sorted: apple at %d, zebra at %d", appleIdx, zebraIdx)
	}
}

func TestShowInvalidEnvFlag(t *testing.T) {
	_, err := run(t, "show",
		"--env-file", envFile(t, "env: {}\n"),
		"--env", "no-equals",
	)
	if err == nil {
		t.Error("expected error for missing = in --env flag")
	}
}
