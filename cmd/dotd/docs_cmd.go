package main

import (
	"fmt"
	"io"
	"io/fs"
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

// newDocsCmd builds the `docs` command. Three ways in: --list shows the
// available topics, `docs <topic>` prints one page, --full prints the
// complete machine-readable reference (an llms-full-style blob): a
// provenance header, the embedded prose, then the full CLI reference.
// Bare `dotd docs` falls through to cobra's own help.
func newDocsCmd() *cobra.Command {
	var full, list bool
	cmd := &cobra.Command{
		Use:   "docs [topic]",
		Short: "Print embedded documentation",
		Long: `Print dot-dagger's documentation, embedded in the binary.

With a topic argument, prints that single page. Topics are addressed by
path (concepts/conditions) or, when unambiguous, by name (conditions).
Run --list to see every topic.

With --full, prints the complete machine-readable reference to stdout: every
concept and reference page plus the full CLI reference, as one llms-full-style
blob — the form intended for agents and tooling. Offline; no doc-site needed.

Examples:
  dotd docs --list                  # list available topics
  dotd docs conditions              # print one page by name
  dotd docs reference/annotations   # by path when the name is ambiguous
  dotd docs --full                  # complete reference (for agents)
  dotd docs --full | less           # page it`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			topics, err := docs.Topics(dotdagger.DocsFS)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			comps := make([]string, 0, len(topics))
			for _, t := range topics {
				comps = append(comps, t.Slug+"\t"+t.Title)
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (full || list) {
				return asUsageError(fmt.Errorf("a topic argument cannot be combined with --full or --list"))
			}
			out := cmd.OutOrStdout()
			switch {
			case list:
				return runDocsList(out)
			case len(args) == 1:
				return runDocsTopic(out, args[0])
			case full:
				fmt.Fprintf(out, "# dotd %s — embedded reference\n\n", version)
				prose, err := docs.RenderProse(dotdagger.DocsFS)
				if err != nil {
					return fmt.Errorf("render docs: %w", err)
				}
				fmt.Fprint(out, prose)
				fmt.Fprint(out, renderCommandRef(cmd.Root()))
				return nil
			default:
				return cmd.Help()
			}
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "print the complete machine-readable reference (for agents)")
	cmd.Flags().BoolVar(&list, "list", false, "list available documentation topics")
	cmd.MarkFlagsMutuallyExclusive("full", "list")
	return cmd
}

// runDocsList prints one topic per line: slug column, then title.
func runDocsList(w io.Writer) error {
	topics, err := docs.Topics(dotdagger.DocsFS)
	if err != nil {
		return fmt.Errorf("list docs: %w", err)
	}
	// Column width follows the longest slug — no hand-maintained constant.
	width := 0
	for _, t := range topics {
		width = max(width, len(t.Slug))
	}
	for _, t := range topics {
		fmt.Fprintf(w, "%-*s  %s\n", width, t.Slug, t.Title)
	}
	return nil
}

// runDocsTopic prints the single page query resolves to, or a usage error
// when the query is unknown or ambiguous.
func runDocsTopic(w io.Writer, query string) error {
	topics, err := docs.Topics(dotdagger.DocsFS)
	if err != nil {
		return fmt.Errorf("resolve docs topic: %w", err)
	}
	matches := docs.Match(topics, query)
	switch len(matches) {
	case 1:
		body, err := fs.ReadFile(dotdagger.DocsFS, matches[0].Path)
		if err != nil {
			return fmt.Errorf("read docs topic %q: %w", matches[0].Slug, err)
		}
		_, err = w.Write(body)
		return err
	case 0:
		return &usageError{err: &hintError{
			err:  fmt.Errorf("unknown docs topic %q", query),
			hint: "run 'dotd docs --list' to see available topics",
		}}
	default:
		slugs := make([]string, len(matches))
		for i, m := range matches {
			slugs[i] = m.Slug
		}
		return &usageError{err: &hintError{
			err:  fmt.Errorf("docs topic %q is ambiguous: %s", query, strings.Join(slugs, ", ")),
			hint: fmt.Sprintf("pass the full path, e.g. 'dotd docs %s'", slugs[0]),
		}}
	}
}
