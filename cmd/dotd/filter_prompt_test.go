package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/rocne/dot-dagger/internal/pipeline"
)

func makeFilterNode(name, when string) pipeline.RawNode {
	return pipeline.RawNode{
		Path:          "/dots/" + name,
		LogicalName:   name,
		EffectiveWhen: when,
	}
}

func TestFilterWithPrompt_NonTTY_MissingKey_ReturnsAnnotatedError(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("a", "context=work")}
	resolved := map[string]string{} // context missing

	_, err := filterWithPrompt(nodes, resolved, false)
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected error to mention key 'context', got: %v", err)
	}
}

func TestFilterWithPrompt_NonTTY_NoMissingKeys_ReturnsActiveNodes(t *testing.T) {
	nodes := []pipeline.RawNode{
		makeFilterNode("base", ""),
		makeFilterNode("work", "context=work"),
		makeFilterNode("personal", "context=personal"),
	}
	resolved := map[string]string{"context": "work"}

	active, err := filterWithPrompt(nodes, resolved, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active nodes (base + work), got %d", len(active))
	}
}

func TestFilterWithPrompt_TTY_NoMissingKeys_DoesNotPrompt(t *testing.T) {
	// isTTY=true but no missing keys — should proceed without prompting.
	nodes := []pipeline.RawNode{makeFilterNode("base", "")}
	resolved := map[string]string{}

	active, err := filterWithPrompt(nodes, resolved, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active node, got %d", len(active))
	}
}

func TestFilterWithPrompt_TTY_UserAbort_ReturnsErrUserAborted(t *testing.T) {
	// Inject a stub prompter that simulates the user pressing Ctrl-C.
	// Verifies filterWithPrompt returns errUserAborted (not os.Exit).
	orig := prompter
	defer func() { prompter = orig }()
	prompter = func(keys []string) (map[string]string, error) {
		return nil, huh.ErrUserAborted
	}

	// Node with a missing key so the TTY branch reaches the prompter.
	nodes := []pipeline.RawNode{makeFilterNode("work", "context=work")}
	resolved := map[string]string{} // context is missing

	_, err := filterWithPrompt(nodes, resolved, true)
	if !errors.Is(err, errUserAborted) {
		t.Errorf("expected errUserAborted, got %v", err)
	}
}

// TestFilterWithPrompt_TTY_HappyPath verifies that when the stub prompter
// returns values for missing keys, those values are augmented into the env
// and the nodes that match the supplied values filter through.
func TestFilterWithPrompt_TTY_HappyPath(t *testing.T) {
	orig := prompter
	defer func() { prompter = orig }()

	// Stub returns values for the two missing keys.
	prompter = func(keys []string) (map[string]string, error) {
		result := map[string]string{}
		for _, k := range keys {
			switch k {
			case "os":
				result[k] = "linux"
			case "context":
				result[k] = "work"
			default:
				result[k] = "stubval"
			}
		}
		return result, nil
	}

	nodes := []pipeline.RawNode{
		makeFilterNode("base", ""),
		makeFilterNode("linux-work", "os=linux"),
		makeFilterNode("macos-only", "os=macos"),
		makeFilterNode("work-context", "context=work"),
	}
	// Both "os" and "context" are missing from resolved.
	resolved := map[string]string{}

	active, err := filterWithPrompt(nodes, resolved, true)
	if err != nil {
		t.Fatalf("filterWithPrompt error = %v", err)
	}

	names := map[string]bool{}
	for _, n := range active {
		names[n.LogicalName] = true
	}

	if !names["base"] {
		t.Error("expected 'base' (unconditional) in active nodes")
	}
	if !names["linux-work"] {
		t.Error("expected 'linux-work' (os=linux) in active nodes after prompt filled os=linux")
	}
	if !names["work-context"] {
		t.Error("expected 'work-context' (context=work) in active nodes after prompt filled context=work")
	}
	if names["macos-only"] {
		t.Error("expected 'macos-only' (os=macos) NOT in active nodes when os was set to 'linux'")
	}
}

// TestPrintPersistHint verifies that printPersistHint writes the YAML
// representation of the filled map to the writer.
func TestPrintPersistHint(t *testing.T) {
	filled := map[string]string{
		"os":      "linux",
		"context": "work",
	}

	var buf bytes.Buffer
	printPersistHint(&buf, filled)
	out := buf.String()

	if !strings.Contains(out, "os") {
		t.Errorf("expected 'os' key in persist hint output: %q", out)
	}
	if !strings.Contains(out, "linux") {
		t.Errorf("expected 'linux' value in persist hint output: %q", out)
	}
	if !strings.Contains(out, "context") {
		t.Errorf("expected 'context' key in persist hint output: %q", out)
	}
	if !strings.Contains(out, "work") {
		t.Errorf("expected 'work' value in persist hint output: %q", out)
	}
	// Output must mention the env file name (hint instructs the user where to persist).
	if !strings.Contains(out, "env.yaml") {
		t.Errorf("expected 'env.yaml' in persist hint output: %q", out)
	}
}
