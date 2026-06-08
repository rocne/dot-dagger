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
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "plain=") && strings.Contains(line, "[$(") {
			t.Errorf("plain value should not have annotation, got line: %q", line)
		}
	}
}
