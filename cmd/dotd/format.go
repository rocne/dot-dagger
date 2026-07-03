package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

// plural renders "1 word" / "N words" — every user-facing count goes
// through it (no hand-rolled "(s)" suffixes).
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// bannerf prints a command's opening banner: bold command path, em-dash,
// subtitle. The name comes from cobra so it can never drift from the
// registered command.
func bannerf(w io.Writer, cmd *cobra.Command, subtitle string) {
	fmt.Fprintf(w, "%s — %s\n", ui.Header(cmd.CommandPath()), subtitle)
}

// addJSONFlag registers the standard --json flag into target — one owner
// for the flag name and help text across all list/show commands.
func addJSONFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "json", false, "output JSON array")
}

// writeJSON renders v as two-space-indented JSON to w — the single owner of
// the CLI's machine-readable output format. Every --json path routes through
// it so indentation and encoding can never drift between commands.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
