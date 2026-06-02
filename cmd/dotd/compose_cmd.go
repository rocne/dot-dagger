package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
			ordered, err := cfg.walkOrdered()
			if err != nil {
				return err
			}
			for _, n := range ordered {
				if !n.HasCompose() {
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), pipeline.ComposeFileName(n.Path))
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
			ordered, err := cfg.walkOrdered()
			if err != nil {
				return err
			}
			actOpts, err := buildActOptions(cfg, true)
			if err != nil {
				return err
			}
			res, err := pipeline.Act(ordered, actOpts)
			if err != nil {
				return fmt.Errorf("compose check: %w", err)
			}

			var hasStale bool
			for _, gen := range res.Generated {
				if gen.Path == "" {
					continue
				}
				existing, readErr := os.ReadFile(gen.Path)
				if errors.Is(readErr, fs.ErrNotExist) {
					ui.Missingf(cmd.OutOrStdout(), "%s", filepath.Base(gen.Path))
					hasStale = true
					continue
				}
				if readErr != nil {
					return fmt.Errorf("compose check: read %s: %w", gen.Path, readErr)
				}
				if !bytes.Equal(existing, gen.Content) {
					ui.Wrongf(cmd.OutOrStdout(), "%s", filepath.Base(gen.Path))
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

