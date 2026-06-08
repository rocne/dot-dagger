package main

import (
	"bufio"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptPath_acceptsDefault(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n")) // empty input = accept default
	home := "/home/user"
	got, err := promptPath(io.Discard, reader, "Label", "Desc", "", "/resolved/default", home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs("/resolved/default")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPromptPath_existingValOverridesDefault(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	home := "/home/user"
	got, err := promptPath(io.Discard, reader, "Label", "Desc", "/existing", "/resolved/default", home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs("/existing")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPromptPath_userTypesValue(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("/custom/path\n"))
	home := "/home/user"
	got, err := promptPath(io.Discard, reader, "Label", "Desc", "", "/default", home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs("/custom/path")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPromptPath_expandsTilde(t *testing.T) {
	home := "/home/user"
	reader := bufio.NewReader(strings.NewReader("\n"))
	got, err := promptPath(io.Discard, reader, "Label", "Desc", "", "~/dots", home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/user/dots"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
