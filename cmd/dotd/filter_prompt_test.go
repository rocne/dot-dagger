package main

import (
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
