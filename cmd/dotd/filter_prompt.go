package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/pipeline"
	yaml "gopkg.in/yaml.v3"
)

// filterWithPrompt wraps pipeline.Filter with TTY-aware missing-key prompting.
// Non-TTY: identical to Filter + annotateKeyError.
// TTY with missing keys: prompts for all missing keys, then runs Filter with augmented env.
func filterWithPrompt(nodes []pipeline.RawNode, resolved map[string]string, isTTY bool) ([]pipeline.RawNode, error) {
	if !isTTY {
		active, err := pipeline.Filter(nodes, resolved)
		return active, annotateKeyError(err)
	}

	missing, err := pipeline.CollectMissingKeys(nodes, resolved)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		active, err := pipeline.Filter(nodes, resolved)
		return active, annotateKeyError(err)
	}

	filled, err := promptMissingKeys(missing)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			os.Exit(1)
		}
		return nil, err
	}

	printPersistHint(os.Stderr, filled)

	augmented := maps.Clone(resolved)
	for k, v := range filled {
		augmented[k] = v
	}

	active, err := pipeline.Filter(nodes, augmented)
	return active, annotateKeyError(err)
}

func promptMissingKeys(keys []string) (map[string]string, error) {
	vals := make([]string, len(keys))
	fields := make([]huh.Field, len(keys))
	for i, k := range keys {
		fields[i] = huh.NewInput().
			Title(fmt.Sprintf("env key %q is not set", k)).
			Value(&vals[i]).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("value required")
				}
				return nil
			})
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return nil, err
	}
	filled := make(map[string]string, len(keys))
	for i, k := range keys {
		filled[k] = vals[i]
	}
	return filled, nil
}

func printPersistHint(w io.Writer, filled map[string]string) {
	out, err := yaml.Marshal(filled)
	if err != nil {
		// Fallback to raw output if marshaling fails (shouldn't happen for string maps).
		fmt.Fprintf(w, "\nHint: to persist, add to %s:\n", ecosystem.EnvFileName)
		for k, v := range filled {
			fmt.Fprintf(w, "  %s: %s\n", k, v)
		}
		return
	}
	fmt.Fprintln(w, "\nHint: to persist, add to env.yaml:")
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

// isTTYStdin returns true when os.Stdin is an interactive terminal.
func isTTYStdin() bool {
	return term.IsTerminal(os.Stdin.Fd())
}
