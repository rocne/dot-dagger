package main

import (
	"bytes"
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
	for _, want := range []string{"$bin", "$config", "home", "init.sh", "dotfiles", "config.yaml", "env.yaml", "/h"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("paths output missing %q:\n%s", want, out.String())
		}
	}
}
