package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func makeFilterNode(name, when string) pipeline.RawNode {
	return pipeline.RawNode{
		Path:          "/dots/" + name,
		LogicalName:   name,
		EffectiveWhen: when,
	}
}

// testCmd returns a cobra.Command wired to the given stdin.
// stdout and stderr are discarded so test output stays clean.
func testCmd(stdin io.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetIn(stdin)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

func TestFilterWithPrompt_NonTTY_MissingKey_ReturnsAnnotatedError(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("a", "context=work")}
	resolved := map[string]string{} // context missing

	_, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, false)
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

	active, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active nodes (base + work), got %d", len(active))
	}
}

func TestFilterWithPrompt_TTY_NoMissingKeys_DoesNotPrompt(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("base", "")}
	resolved := map[string]string{}

	active, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active node, got %d", len(active))
	}
}

// TestFilterWithPrompt_TTY_HappyPath verifies that when the user provides values
// for missing keys via accessible-mode stdin, those values augment the env and
// the correct nodes filter through.
func TestFilterWithPrompt_TTY_HappyPath(t *testing.T) {
	nodes := []pipeline.RawNode{
		makeFilterNode("base", ""),
		makeFilterNode("linux-work", "os=linux"),
		makeFilterNode("macos-only", "os=macos"),
		makeFilterNode("work-context", "context=work"),
	}
	resolved := map[string]string{} // both "os" and "context" missing

	// Accessible mode: huh prompts for each missing key in order.
	// promptMissingKeys calls CollectMissingKeys then promptInputs.
	// CollectMissingKeys returns keys in encounter order (not sorted).
	// Nodes iterate: "linux-work" has os=linux → "os" collected first;
	// "work-context" has context=work → "context" collected second.
	// Input: "linux" for os, "work" for context.
	stdin := strings.NewReader("linux\nwork\n")

	active, err := filterWithPrompt(testCmd(stdin), nodes, resolved, true)
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
		t.Error("expected 'linux-work' in active nodes")
	}
	if !names["work-context"] {
		t.Error("expected 'work-context' in active nodes")
	}
	if names["macos-only"] {
		t.Error("expected 'macos-only' NOT in active nodes")
	}
}

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
	if !strings.Contains(out, "env.yaml") {
		t.Errorf("expected 'env.yaml' in persist hint output: %q", out)
	}
}
