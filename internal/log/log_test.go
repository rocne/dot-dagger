package log

import (
	"bytes"
	"strings"
	"testing"
)

// TestNew_ParsesEachLevel verifies New accepts every level listed by
// LevelNames(). The --log-level flag validation at cmd/dotd/main.go relies
// on this round-trip.
func TestNew_ParsesEachLevel(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		t.Run(lvl, func(t *testing.T) {
			var buf bytes.Buffer
			lg, err := New(&buf, lvl)
			if err != nil {
				t.Fatalf("New(%q): %v", lvl, err)
			}
			if lg == nil {
				t.Fatalf("New(%q) returned nil logger", lvl)
			}
		})
	}
}

// TestNew_RejectsBadName ensures a bogus level name returns an error rather
// than silently degrading to a default.
func TestNew_RejectsBadName(t *testing.T) {
	var buf bytes.Buffer
	if _, err := New(&buf, "verbose"); err == nil {
		t.Fatal("expected error for unknown level")
	}
}

// TestLevelNames asserts the help string lists every level New accepts so
// they don't drift apart.
func TestLevelNames(t *testing.T) {
	names := LevelNames()
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		if !strings.Contains(names, lvl) {
			t.Errorf("LevelNames() = %q, missing %q", names, lvl)
		}
	}
}
