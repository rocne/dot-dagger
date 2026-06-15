package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRenderCommandRef_IncludesVisibleSkipsHelpers(t *testing.T) {
	root := &cobra.Command{Use: "dotd"}
	root.AddCommand(&cobra.Command{Use: "apply", Short: "apply things", Long: "Apply the dotfiles."})
	root.AddCommand(&cobra.Command{Use: "secret", Short: "x", Hidden: true})
	root.AddCommand(&cobra.Command{Use: "completion", Short: "gen completion"})

	out := renderCommandRef(root)

	if !strings.Contains(out, "dotd apply") {
		t.Error("CLI reference missing 'dotd apply'")
	}
	if !strings.Contains(out, "Apply the dotfiles.") {
		t.Error("CLI reference missing apply Long text")
	}
	if strings.Contains(out, "secret") {
		t.Error("hidden command leaked into CLI reference")
	}
	if strings.Contains(out, "dotd completion") {
		t.Error("completion command leaked into CLI reference")
	}
}
