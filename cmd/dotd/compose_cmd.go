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
		Long: `Compose targets are directories whose contents 'dotd apply' assembles into
a single generated file. A directory named "<name>.d" is a compose target;
its files are concatenated in DAG order into "<name>".`,
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
		Long: `List the generated file path for each active compose target.

Filtered by @when predicates against the resolved env.

Examples:
  dotd compose list
  dotd compose list --env context=work`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
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
		Long: `Compare the on-disk generated file for each compose target against the
freshly assembled content. Exits non-zero if any target is missing or stale.

Examples:
  dotd compose check
  dotd compose check && echo "all clean"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
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

			errOut := cmd.ErrOrStderr()
			var hasStale bool
			for _, gen := range res.Generated {
				if gen.Path == "" {
					continue
				}
				existing, readErr := os.ReadFile(gen.Path)
				if errors.Is(readErr, fs.ErrNotExist) {
					ui.Missingf(errOut, "%s", filepath.Base(gen.Path))
					hasStale = true
					continue
				}
				if readErr != nil {
					return fmt.Errorf("compose check: read %s: %w", gen.Path, readErr)
				}
				if !bytes.Equal(existing, gen.Content) {
					ui.Wrongf(errOut, "%s", filepath.Base(gen.Path))
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

