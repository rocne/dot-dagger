package main

import (
	"fmt"
	"strings"

	dotdagger "github.com/rocne/dot-dagger"
	"github.com/rocne/dot-dagger/internal/docs"
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

// newDocsCmd builds the `docs` command. With --full it prints the complete
// machine-readable reference (an llms-full-style blob): a provenance header,
// the embedded prose, then the full CLI reference. Without --full it falls
// through to cobra's own help (which lists --full); per-topic human output is
// a future follow-up that reuses RenderProse.
func newDocsCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Print embedded documentation",
		Long: `Print dot-dagger's documentation, embedded in the binary.

With --full, prints the complete machine-readable reference to stdout: every
concept and reference page plus the full CLI reference, as one llms-full-style
blob — the form intended for agents and tooling. Offline; no doc-site needed.

Examples:
  dotd docs --full              # complete reference (for agents)
  dotd docs --full | less       # page it
  dotd docs --full > dotd.txt   # capture to a file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !full {
				return cmd.Help()
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# dotd %s — embedded reference\n\n", version)
			prose, err := docs.RenderProse(dotdagger.DocsFS)
			if err != nil {
				return fmt.Errorf("render docs: %w", err)
			}
			fmt.Fprint(out, prose)
			fmt.Fprint(out, renderCommandRef(cmd.Root()))
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "print the complete machine-readable reference (for agents)")
	return cmd
}
