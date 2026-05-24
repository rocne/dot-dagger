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
