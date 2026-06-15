package main

import (
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
