package main

import (
	"bytes"
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
	cmd := newDocsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("full", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "embedded reference") {
		t.Error("missing provenance header")
	}
	if !strings.Contains(s, "# === docs/concepts/") {
		t.Error("missing embedded concepts section")
	}
	if !strings.Contains(s, "CLI Reference") {
		t.Error("missing CLI reference section")
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
