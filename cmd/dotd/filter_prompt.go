package main

import (
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// filterWithPrompt wraps pipeline.Filter with TTY-aware missing-key prompting.
// tty=false: identical to Filter + annotateKeyError (non-interactive path).
// tty=true with missing keys: prompts for all missing keys via cmd's I/O,
// then re-runs Filter with the augmented env.
// Call site: filterWithPrompt(cmd, nodes, resolved, isTTY(cmd.InOrStdin()))
func filterWithPrompt(cmd *cobra.Command, nodes []pipeline.RawNode, resolved map[string]string, tty bool) ([]pipeline.RawNode, error) {
	if !tty {
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

	filled, err := promptMissingKeys(cmd, missing)
	if err != nil {
		return nil, err
	}

	printPersistHint(cmd.ErrOrStderr(), filled)

	augmented := maps.Clone(resolved)
	for k, v := range filled {
		augmented[k] = v
	}

	active, err := pipeline.Filter(nodes, augmented)
	return active, annotateKeyError(err)
}

func promptMissingKeys(cmd *cobra.Command, keys []string) (map[string]string, error) {
	prompts := make([]inputPrompt, len(keys))
	for i, k := range keys {
		prompts[i] = inputPrompt{
			Key:   k,
			Title: fmt.Sprintf("env key %q is not set", k),
		}
	}
	return promptInputs(cmd, prompts)
}

func printPersistHint(w io.Writer, filled map[string]string) {
	fmt.Fprintf(w, "\nhint: to persist, add to %s:\n", ecosystem.EnvFileName)
	out, err := yaml.Marshal(filled)
	if err != nil {
		// yaml.Marshal on map[string]string is infallible; this branch is unreachable in practice
		for k, v := range filled {
			fmt.Fprintf(w, "  %s: %s\n", k, v)
		}
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
}
