package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newComposeCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Manage compose targets (assembled fragment files)",
	}
	cmd.AddCommand(
		newComposeListCmd(cfg),
		newComposeCheckCmd(cfg),
	)
	return cmd
}

func newComposeListCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active compose targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, _, err := pipeline.Walk(cfg.files)
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
			active, err := pipeline.Filter(nodes, resolved)
			if err != nil {
				return annotateKeyError(err)
			}
			for _, n := range active {
				if !hasComposeAction(n) {
					continue
				}
				genName := composeGenName(n)
				fmt.Fprintln(cmd.OutOrStdout(), genName)
			}
			return nil
		},
	}
}

func newComposeCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check compose targets for staleness",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, _, err := pipeline.Walk(cfg.files)
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
			active, err := pipeline.Filter(nodes, resolved)
			if err != nil {
				return annotateKeyError(err)
			}
			ordered, err := pipeline.Order(active)
			if err != nil {
				return fmt.Errorf("order: %w", err)
			}
			res, err := pipeline.Act(ordered, buildActOptions(cfg, true))
			if err != nil {
				return fmt.Errorf("act: %w", err)
			}

			var hasStale bool
			for _, gen := range res.Generated {
				if gen.Path == "" {
					continue
				}
				existing, readErr := os.ReadFile(gen.Path)
				if errors.Is(readErr, fs.ErrNotExist) {
					fmt.Fprintf(cmd.OutOrStdout(), "missing: %s\n", filepath.Base(gen.Path))
					hasStale = true
					continue
				}
				if readErr != nil {
					return fmt.Errorf("read %s: %w", gen.Path, readErr)
				}
				if !bytes.Equal(existing, gen.Content) {
					fmt.Fprintf(cmd.OutOrStdout(), "stale: %s\n", filepath.Base(gen.Path))
					hasStale = true
				}
			}

			if hasStale {
				cfg.log.Warn("compose: stale generated files found; run 'dotd apply' to update")
			} else {
				cfg.log.Infof("%s all compose targets up-to-date", ui.Header("compose:"))
			}
			return nil
		},
	}
}

func hasComposeAction(n pipeline.RawNode) bool {
	for _, a := range n.Actions {
		if a.Type == "compose" {
			return true
		}
	}
	return false
}

func composeGenName(n pipeline.RawNode) string {
	return genNameFromDir(n.Path)
}

// genNameFromDir derives the generated filename from a compose target dir path.
// "dot-shellrc-extras.sh.d" → "shellrc-extras.sh", "dot-tmux.conf.d" → "tmux.conf"
func genNameFromDir(dirPath string) string {
	base := strings.TrimPrefix(filepath.Base(dirPath), "dot-")
	return strings.TrimSuffix(base, ".d")
}
