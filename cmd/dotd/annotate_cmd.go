package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/spf13/cobra"
)

func newAnnotateCmd(rootCfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "annotate <file>",
		Short: "Interactively add or edit dotd annotations in a file's header",
		Long: `Interactively add or edit dotd annotations in a file's header block.

Opens a menu to select annotation types. Existing annotations are pre-loaded.
Writes the updated annotation block atomically when confirmed.

Works in both interactive terminals and non-interactive contexts (accessible mode).
The file must be inside the dotfiles directory (--files).

For the full annotation reference, run: dotd concepts

Examples:
  dotd annotate shellrc/base.sh
  dotd annotate conf/dot-gitconfig`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnnotate(cmd, rootCfg, args[0])
		},
	}
}

// stagedEntry holds one annotation to be written.
type stagedEntry struct {
	Type  annotation.AnnotationType
	Value string
}

func runAnnotate(cmd *cobra.Command, cfg *config, fileArg string) error {
	absFile, err := filepath.Abs(fileArg)
	if err != nil {
		return fmt.Errorf("annotate: resolve path: %w", err)
	}

	stat, err := os.Stat(absFile)
	if err != nil {
		return fmt.Errorf("annotate: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("annotate: %s is not a regular file", absFile)
	}

	rel, err := filepath.Rel(cfg.files, absFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("annotate: %s is outside the dotfiles directory %s", absFile, cfg.files)
	}

	// Scan existing annotations.
	f, err := os.Open(absFile)
	if err != nil {
		return fmt.Errorf("annotate: open: %w", err)
	}
	scanned, err := annotation.Scan(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("annotate: scan: %w", err)
	}

	// Build registry key lookup.
	byKey := make(map[string]annotation.AnnotationType, len(annotation.Registry))
	for _, t := range annotation.Registry {
		byKey[t.Key()] = t
	}

	// Pre-populate staged list and preserved (unknown keys).
	var staged []stagedEntry
	var preserved []string
	for _, a := range scanned {
		if t, ok := byKey[a.Key]; ok {
			staged = append(staged, stagedEntry{Type: t, Value: a.Args})
		} else {
			if a.Args != "" {
				preserved = append(preserved, fmt.Sprintf("# @%s(%s)", a.Key, a.Args))
			} else {
				preserved = append(preserved, fmt.Sprintf("# @%s", a.Key))
			}
		}
	}

	out := cmd.OutOrStdout()

	// Menu loop.
	for {
		options := make([]string, len(annotation.Registry)+1)
		for i, t := range annotation.Registry {
			entries := stagedEntriesFor(staged, t.Key())
			switch {
			case len(entries) == 0:
				options[i] = t.Label()
			case t.Kind() == annotation.KindBool:
				options[i] = t.Label() + "  [set]"
			case len(entries) == 1:
				options[i] = t.Label() + "  [" + entries[0].Value + "]"
			default:
				options[i] = fmt.Sprintf("%s  [%d]", t.Label(), len(entries))
			}
		}
		options[len(annotation.Registry)] = "Done"

		idx, err := promptMenu(cmd, "Select annotation", options)
		if err != nil {
			return fmt.Errorf("annotate: menu: %w", err)
		}
		if idx == len(annotation.Registry) {
			break
		}

		t := annotation.Registry[idx]
		entries := stagedEntriesFor(staged, t.Key())

		switch t.Kind() {
		case annotation.KindBool:
			set, err := promptBool(cmd, t.Label()+"?", t.Description(), "Set", "Remove", len(entries) > 0)
			if err != nil {
				return fmt.Errorf("annotate: confirm: %w", err)
			}
			staged = removeStagedKey(staged, t.Key())
			if set {
				staged = append(staged, stagedEntry{Type: t, Value: ""})
				fmt.Fprintf(out, "  → %s\n", t.Format(""))
			} else {
				fmt.Fprintf(out, "  → (removed)\n")
			}

		case annotation.KindChoice:
			opts := append(append([]string{}, t.Options()...), "none")
			chosen, err := promptSelect(cmd, t.Label(), t.Description(), opts)
			if err != nil {
				return fmt.Errorf("annotate: select: %w", err)
			}
			if chosen == "none" {
				staged = removeStagedKey(staged, t.Key())
				fmt.Fprintf(out, "  → (removed)\n")
			} else if len(entries) == 1 {
				staged = replaceStagedKey(staged, t.Key(), chosen)
				fmt.Fprintf(out, "  → %s\n", t.Format(chosen))
			} else {
				staged = removeStagedKey(staged, t.Key())
				staged = append(staged, stagedEntry{Type: t, Value: chosen})
				fmt.Fprintf(out, "  → %s\n", t.Format(chosen))
			}

		case annotation.KindText:
			prefill := ""
			if len(entries) == 1 {
				prefill = entries[0].Value
			}
			fmt.Fprintln(out, t.Description())
			val, err := promptInput(cmd, t.Label(), "(enter value, or clear to remove)", prefill, t.Validate)
			if err != nil {
				return fmt.Errorf("annotate: input: %w", err)
			}
			switch {
			case val == "":
				staged = removeStagedKey(staged, t.Key())
				fmt.Fprintf(out, "  → (removed)\n")
			case len(entries) == 1:
				staged = replaceStagedKey(staged, t.Key(), val)
				fmt.Fprintf(out, "  → %s\n", t.Format(val))
			default:
				staged = append(staged, stagedEntry{Type: t, Value: val})
				fmt.Fprintf(out, "  → %s\n", t.Format(val))
			}
		}
	}

	// Confirm + write.
	fmt.Fprintf(out, "\nAnnotations to write:\n")
	if len(staged) == 0 {
		fmt.Fprintf(out, "  (none — all known annotations will be removed)\n")
	}
	for _, e := range staged {
		fmt.Fprintf(out, "  %s\n", e.Type.Format(e.Value))
	}

	confirmed, err := promptBool(cmd,
		fmt.Sprintf("Write these annotations to %s?", filepath.Base(absFile)),
		"", "Yes", "No, cancel", false)
	if err != nil {
		return fmt.Errorf("annotate: confirm: %w", err)
	}
	if !confirmed {
		return nil
	}

	lines := make([]string, len(staged))
	for i, e := range staged {
		lines[i] = e.Type.Format(e.Value)
	}
	return annotation.Write(absFile, preserved, lines)
}

// stagedEntriesFor returns all staged entries matching key.
func stagedEntriesFor(staged []stagedEntry, key string) []stagedEntry {
	var out []stagedEntry
	for _, e := range staged {
		if e.Type.Key() == key {
			out = append(out, e)
		}
	}
	return out
}

// removeStagedKey removes all staged entries for key.
func removeStagedKey(staged []stagedEntry, key string) []stagedEntry {
	out := staged[:0]
	for _, e := range staged {
		if e.Type.Key() != key {
			out = append(out, e)
		}
	}
	return out
}

// replaceStagedKey replaces the value of the first (and assumed only) entry for key.
func replaceStagedKey(staged []stagedEntry, key, value string) []stagedEntry {
	for i, e := range staged {
		if e.Type.Key() == key {
			staged[i].Value = value
			return staged
		}
	}
	return staged
}
