package walk

import (
	"os"
	"path/filepath"
	"testing"
)

// mkTree creates a temporary dotfiles tree for testing.
// files maps relative paths to file contents.
func mkTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWalkKinds(t *testing.T) {
	root := mkTree(t, map[string]string{
		"scripts/base.sh":         "# @when os=linux\n",
		"conf/dot-gitconfig":      "",
		"bin/my-tool":             "",
		"other/readme.txt":        "",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	kindOf := func(rel string) Kind {
		full := filepath.Join(root, filepath.FromSlash(rel))
		for _, n := range nodes {
			if n.Path == full {
				return n.Kind
			}
		}
		t.Fatalf("node not found: %s", rel)
		return KindOther
	}

	if kindOf("scripts/base.sh") != KindScript {
		t.Error("scripts/base.sh: want KindScript")
	}
	if kindOf("conf/dot-gitconfig") != KindConf {
		t.Error("conf/dot-gitconfig: want KindConf")
	}
	if kindOf("bin/my-tool") != KindBin {
		t.Error("bin/my-tool: want KindBin")
	}
	if kindOf("other/readme.txt") != KindOther {
		t.Error("other/readme.txt: want KindOther")
	}
}

func TestWalkSpecialDirNesting(t *testing.T) {
	// scripts/conf/ should NOT be treated as a conf dir (inside special dir already).
	root := mkTree(t, map[string]string{
		"scripts/helpers.sh":        "",
		"scripts/conf/dot-tmux.conf": "",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	for _, n := range nodes {
		if filepath.Base(n.Path) == "dot-tmux.conf" && n.Kind == KindConf {
			t.Error("scripts/conf/dot-tmux.conf: should NOT be KindConf (nested special dir)")
		}
	}
}

func TestWalkTopicGrouped(t *testing.T) {
	// tmux/scripts/ should be KindScript.
	root := mkTree(t, map[string]string{
		"tmux/scripts/helpers.sh": "",
		"tmux/conf/dot-tmux.conf": "",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	kindOf := func(rel string) Kind {
		full := filepath.Join(root, filepath.FromSlash(rel))
		for _, n := range nodes {
			if n.Path == full {
				return n.Kind
			}
		}
		t.Fatalf("node not found: %s", rel)
		return KindOther
	}

	if kindOf("tmux/scripts/helpers.sh") != KindScript {
		t.Error("tmux/scripts/helpers.sh: want KindScript")
	}
	if kindOf("tmux/conf/dot-tmux.conf") != KindConf {
		t.Error("tmux/conf/dot-tmux.conf: want KindConf")
	}
}

func TestWalkLogicalName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"scripts/helpers.sh", "scripts.helpers"},
		{"tmux/scripts/helpers.sh", "tmux.scripts.helpers"},
		{"nosync-work/scripts/aliases.sh", "work.scripts.aliases"},
		{"conf/dot-gitconfig", "conf.gitconfig"},
		{"nosync-dot-secrets/api.sh", "secrets.api"},
	}

	files := make(map[string]string)
	for _, tt := range tests {
		files[tt.path] = ""
	}
	root := mkTree(t, files)

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	nameOf := func(rel string) string {
		full := filepath.Join(root, filepath.FromSlash(rel))
		for _, n := range nodes {
			if n.Path == full {
				return n.LogicalName
			}
		}
		t.Fatalf("node not found: %s", rel)
		return ""
	}

	for _, tt := range tests {
		got := nameOf(tt.path)
		if got != tt.want {
			t.Errorf("logicalName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestWalkAnnotations(t *testing.T) {
	root := mkTree(t, map[string]string{
		"scripts/base.sh": "#!/bin/bash\n# @when os=linux\n# @after scripts.other\nexport FOO=1\n",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	n := nodes[0]
	if n.EffectiveWhen != "os=linux" {
		t.Errorf("EffectiveWhen = %q, want %q", n.EffectiveWhen, "os=linux")
	}
}

func TestWalkDotRYamlCascade(t *testing.T) {
	root := mkTree(t, map[string]string{
		".dotr.yaml":        "dotd:\n  defaults:\n    when: \"context=work\"\n",
		"scripts/base.sh":   "# @when os=linux\n",
		"scripts/other.sh":  "",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	for _, n := range nodes {
		base := filepath.Base(n.Path)
		switch base {
		case "base.sh":
			// Has own @when — should be combined with cascade.
			if n.EffectiveWhen == "" {
				t.Errorf("base.sh: EffectiveWhen is empty, want combined expression")
			}
		case "other.sh":
			// No own @when — effective when should be the cascade only.
			if n.EffectiveWhen != "context=work" {
				t.Errorf("other.sh: EffectiveWhen = %q, want %q", n.EffectiveWhen, "context=work")
			}
		}
	}
}

func TestWalkNameAnnotation(t *testing.T) {
	root := mkTree(t, map[string]string{
		"tmux/scripts/tmux-helpers-macos.sh": "# @name tmux.scripts.helpers\n",
	})

	nodes, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].LogicalName != "tmux.scripts.helpers" {
		t.Errorf("LogicalName = %q, want tmux.scripts.helpers", nodes[0].LogicalName)
	}
}

func TestLogicalNameFor(t *testing.T) {
	tests := []struct {
		rel  string
		want string
	}{
		{"scripts/helpers.sh", "scripts.helpers"},
		{"conf/dot-gitconfig", "conf.gitconfig"},
		{"nosync-work/scripts/aliases.sh", "work.scripts.aliases"},
		{"nosync-dot-secrets/api.sh", "secrets.api"},
		{"conf/dot-config/tmux/tmux.conf", "conf.config.tmux.tmux"},
	}
	for _, tt := range tests {
		got := logicalNameFor("/root", "/root/"+filepath.FromSlash(tt.rel))
		if got != tt.want {
			t.Errorf("logicalNameFor(%q) = %q, want %q", tt.rel, got, tt.want)
		}
	}
}

func TestCombineWhen(t *testing.T) {
	tests := []struct{ a, b, want string }{
		{"", "", ""},
		{"os=linux", "", "os=linux"},
		{"", "context=work", "context=work"},
		{"os=linux", "context=work", "(os=linux) AND (context=work)"},
	}
	for _, tt := range tests {
		got := combineWhen(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("combineWhen(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}
