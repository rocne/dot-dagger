package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// renderCommandRef walks the command tree under root and renders each command's
// help (usage line, Long or Short, then its LOCAL flags) into a single CLI
// Reference section. Hidden commands and the built-in help/completion commands
// are skipped so the section stays signal. Global/persistent flags are
// deliberately NOT repeated per command — they are inherited by every command
// and are documented once via `dotd --help` and docs/reference/dotd.md;
// repeating the 8-flag block ~20 times would bloat the agent-facing blob for no
// gain. Including the full CLI surface here means an agent gets everything in a
// single call instead of discovering and chaining N `dotd <cmd> --help` calls.
func renderCommandRef(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString("# === CLI Reference ===\n\n")
	b.WriteString("Global flags apply to every command and are documented once " +
		"under `dotd --help` and docs/reference/dotd.md; they are omitted per-command below.\n\n")

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n", sub.CommandPath())
			fmt.Fprintf(&b, "Usage: %s\n\n", sub.UseLine())
			if sub.Long != "" {
				b.WriteString(sub.Long)
			} else {
				b.WriteString(sub.Short)
			}
			b.WriteString("\n\n")
			// LocalFlags() excludes flags inherited from the root (the globals),
			// so this shows only the command's own flags (e.g. docs's --full).
			if usages := sub.LocalFlags().FlagUsages(); strings.TrimSpace(usages) != "" {
				b.WriteString("Flags:\n")
				b.WriteString(usages)
				b.WriteByte('\n')
			}
			walk(sub)
		}
	}
	walk(root)
	return b.String()
}
