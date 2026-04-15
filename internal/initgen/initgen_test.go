package initgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/fileset"
)

func TestGenerateEmpty(t *testing.T) {
	got := Generate(nil, "")
	if !strings.Contains(string(got), "dot-dagger") {
		t.Error("expected header comment in output")
	}
	if strings.Contains(string(got), "PATH=") {
		t.Error("PATH line should not appear when binDir is empty")
	}
	if strings.Contains(string(got), "\n. ") {
		t.Error("source lines should not appear with no nodes")
	}
}

func TestGenerateBinDirPrependedToPath(t *testing.T) {
	got := string(Generate(nil, "/home/user/.local/bin"))
	if !strings.Contains(got, "PATH=") {
		t.Error("expected PATH= line")
	}
	if !strings.Contains(got, "/home/user/.local/bin") {
		t.Error("expected bin dir in PATH line")
	}
}

func TestGenerateSourceLines(t *testing.T) {
	nodes := []fileset.Node{
		{Path: "/dotfiles/scripts/base.sh"},
		{Path: "/dotfiles/scripts/aliases.sh"},
	}
	got := string(Generate(nodes, ""))
	if !strings.Contains(got, ". '/dotfiles/scripts/base.sh'") {
		t.Errorf("missing source line for base.sh in:\n%s", got)
	}
	if !strings.Contains(got, ". '/dotfiles/scripts/aliases.sh'") {
		t.Errorf("missing source line for aliases.sh in:\n%s", got)
	}
}

func TestGenerateOrderPreserved(t *testing.T) {
	nodes := []fileset.Node{
		{Path: "/a.sh"},
		{Path: "/b.sh"},
		{Path: "/c.sh"},
	}
	got := string(Generate(nodes, ""))
	posA := strings.Index(got, "/a.sh")
	posB := strings.Index(got, "/b.sh")
	posC := strings.Index(got, "/c.sh")
	if posA >= posB || posB >= posC {
		t.Errorf("source lines out of order: a=%d b=%d c=%d", posA, posB, posC)
	}
}

func TestGenerateSingleQuoteEscaping(t *testing.T) {
	// Path with embedded single quote.
	nodes := []fileset.Node{
		{Path: "/path/with'quote/script.sh"},
	}
	got := string(Generate(nodes, ""))
	// The embedded quote should be escaped as '\''.
	if !strings.Contains(got, `'\''`) {
		t.Errorf("single quote not escaped in: %s", got)
	}
}

func TestSingleQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/simple/path", "'/simple/path'"},
		{"/path with spaces", "'/path with spaces'"},
		{"/it's/here", "'/it'\\''s/here'"},
	}
	for _, tt := range tests {
		got := singleQuote(tt.in)
		if got != tt.want {
			t.Errorf("singleQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "init.sh")
	content := []byte("# test\n")

	if err := WriteFile(path, content); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}

	// No temp files should remain.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub/dir/init.sh")

	if err := WriteFile(path, []byte("# x\n")); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
