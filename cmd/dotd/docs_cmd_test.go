package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRenderCommandRef_IncludesVisibleSkipsHelpers(t *testing.T) {
	root := &cobra.Command{Use: "dotd"}
	apply := &cobra.Command{Use: "apply", Short: "apply things", Long: "Apply the dotfiles."}
	apply.AddCommand(&cobra.Command{Use: "dry-run", Short: "preview"})
	root.AddCommand(apply)
	root.AddCommand(&cobra.Command{Use: "secret", Short: "x", Hidden: true})
	root.AddCommand(&cobra.Command{Use: "completion", Short: "gen completion"})
	root.AddCommand(&cobra.Command{Use: "list", Short: "List things"})

	out := renderCommandRef(root)

	if !strings.Contains(out, "dotd apply") {
		t.Error("CLI reference missing 'dotd apply'")
	}
	if !strings.Contains(out, "Apply the dotfiles.") {
		t.Error("CLI reference missing apply Long text")
	}
	if !strings.Contains(out, "dotd apply dry-run") {
		t.Error("CLI reference missing 'dotd apply dry-run' — recursion not working")
	}
	if !strings.Contains(out, "List things") {
		t.Error("CLI reference missing 'List things' — Short fallback not working")
	}
	if strings.Contains(out, "## dotd secret") {
		t.Error("hidden command leaked into CLI reference")
	}
	if strings.Contains(out, "## dotd completion") {
		t.Error("completion command leaked into CLI reference")
	}
}

func TestDocsCmd_FullRendersEmbeddedReference(t *testing.T) {
	root := newRootCmd()

	var docsCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "docs" {
			docsCmd = c
			break
		}
	}
	if docsCmd == nil {
		t.Fatal("docs command not registered on root")
	}

	var out bytes.Buffer
	docsCmd.SetOut(&out)

	if err := docsCmd.Flags().Set("full", "true"); err != nil {
		t.Fatal(err)
	}
	if err := docsCmd.RunE(docsCmd, nil); err != nil {
		t.Fatal(err)
	}

	s := out.String()
	if !strings.Contains(s, "embedded reference") {
		t.Error("missing provenance header")
	}
	if !strings.Contains(s, "# === docs/concepts/") {
		t.Error("missing embedded concepts section")
	}
	if !strings.Contains(s, "# === CLI Reference ===") {
		t.Error("missing CLI reference section")
	}
	if !strings.Contains(s, "## dotd apply") {
		t.Errorf("CLI reference does not include real commands from the tree; got:\n%s", s)
	}
}

func TestDocsCmd_ListPrintsTopics(t *testing.T) {
	out, err := run(t, "docs", "--list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"concepts/conditions", "reference/dotd", "getting-started/first-machine"} {
		if !strings.Contains(out, want) {
			t.Errorf("`docs --list` missing topic %q:\n%s", want, out)
		}
	}
}

func TestDocsCmd_TopicRendersSinglePage(t *testing.T) {
	out, err := run(t, "docs", "conditions")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# Conditions") {
		t.Errorf("topic output missing conditions page heading:\n%.400s", out)
	}
	if strings.Contains(out, "# === docs/reference/dotd.md ===") {
		t.Error("topic output leaked other pages — expected a single page")
	}
}

func TestDocsCmd_FullSlugAlsoResolves(t *testing.T) {
	out, err := run(t, "docs", "concepts/conditions")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# Conditions") {
		t.Errorf("full-slug topic output missing page heading:\n%.400s", out)
	}
}

func TestDocsCmd_UnknownTopicIsUsageErrorWithHint(t *testing.T) {
	_, err := run(t, "docs", "no-such-topic")
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
	var ue *usageError
	if !errors.As(err, &ue) {
		t.Errorf("unknown topic should be a usage error (exit 2), got %T: %v", err, err)
	}
	var h Hinter
	if !errors.As(err, &h) || !strings.Contains(h.Hint(), "--list") {
		t.Errorf("unknown topic hint should point at 'dotd docs --list', got: %v", err)
	}
}

func TestDocsCmd_AmbiguousTopicListsCandidates(t *testing.T) {
	// "annotations" exists under both concepts/ and reference/.
	_, err := run(t, "docs", "annotations")
	if err == nil {
		t.Fatal("expected error for ambiguous topic")
	}
	msg := err.Error()
	var h Hinter
	if errors.As(err, &h) {
		msg += " " + h.Hint()
	}
	for _, want := range []string{"concepts/annotations", "reference/annotations"} {
		if !strings.Contains(msg, want) {
			t.Errorf("ambiguous-topic error should list candidate %q, got: %s", want, msg)
		}
	}
}

func TestRootHelp_MentionsDocsFull(t *testing.T) {
	// `--help` short-circuits in cobra before PersistentPreRunE, so this does
	// not run resolvePaths. run() is the existing helper (newRootCmd + buffered
	// out/err + Execute) defined in main_test.go.
	out, err := run(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "docs --full") {
		t.Errorf("`dotd --help` does not mention 'docs --full':\n%s", out)
	}
}
