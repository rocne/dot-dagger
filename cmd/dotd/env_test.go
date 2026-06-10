package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvShowTextUniform verifies that text output is one key=value pair per
// line — no trailing source/expression column. Per-key shell expressions are
// surfaced through --json instead.
func TestEnvShowTextUniform(t *testing.T) {
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

	if !strings.Contains(out, "greeting=hello") {
		t.Errorf("want evaluated value in output, got:\n%s", out)
	}
	if !strings.Contains(out, "plain=world") {
		t.Errorf("want plain value in output, got:\n%s", out)
	}
	if strings.Contains(out, "[$(") || strings.Contains(out, "\t") {
		t.Errorf("text output must be uniform key=value; got:\n%s", out)
	}
}

// TestEnvShowJSONCarriesExpression verifies that --json output records the
// underlying shell expression for keys backed by $(…) values, while plain
// keys omit the expression field.
func TestEnvShowJSONCarriesExpression(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(envFile, []byte("greeting: $(echo hello)\nplain: world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config{envFile: envFile}
	cmd := newEnvShowCmd(cfg)
	cmd.SetArgs([]string{"--json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entries []envShowEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("json unmarshal: %v\nraw: %s", err, buf.String())
	}

	got := map[string]envShowEntry{}
	for _, e := range entries {
		got[e.Key] = e
	}
	if g, ok := got["greeting"]; !ok || g.Value != "hello" || g.Expression != "$(echo hello)" {
		t.Errorf("greeting entry wrong: %+v", g)
	}
	if p, ok := got["plain"]; !ok || p.Value != "world" || p.Expression != "" {
		t.Errorf("plain entry wrong: %+v", p)
	}
}

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
