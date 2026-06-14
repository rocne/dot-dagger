package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPathsCmd_ShowsResolvedAnchors(t *testing.T) {
	cfg := &config{
		home: "/h", binDir: "/h/.local/bin/dot-dagger", configDir: "/h/.config",
		generatedDir: "/h/gen", initFile: "/h/init.sh", files: "/h/dotfiles",
		configPath: "/h/.config/dot-dagger/config.yaml", envFile: "/h/.config/dot-dagger/env.yaml",
	}
	var out bytes.Buffer
	cmd := newPathsCmd(cfg)
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"home", "$bin", "$config", "generated", "init.sh", "dotfiles", "config.yaml", "env.yaml", "/h"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("paths output missing %q:\n%s", want, out.String())
		}
	}
}

func TestPathsCmd_JSONOutput(t *testing.T) {
	cfg := &config{
		home: "/h", binDir: "/h/.local/bin/dot-dagger", configDir: "/h/.config",
		generatedDir: "/h/gen", initFile: "/h/init.sh", files: "/h/dotfiles",
		configPath: "/h/.config/dot-dagger/config.yaml", envFile: "/h/.config/dot-dagger/env.yaml",
	}
	var out bytes.Buffer
	cmd := newPathsCmd(cfg)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rows []pathRow
	if err := json.NewDecoder(&out).Decode(&rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(rows) != 8 {
		t.Errorf("want 8 rows, got %d", len(rows))
	}
}
