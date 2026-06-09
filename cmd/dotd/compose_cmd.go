package main

import (
	"bytes"
	"encoding/json"
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
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active compose targets",
		Long: `List the generated file path for each active compose target.

Filtered by @when predicates against the resolved env.

Examples:
  dotd compose list
  dotd compose list --env context=work
  dotd compose list --json | jq -r '.[]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}
			paths := make([]string, 0)
			for _, n := range ordered {
				if !n.HasCompose() {
					continue
				}
				paths = append(paths, pipeline.ComposeFileName(n.Path))
			}
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(paths)
			}
			for _, p := range paths {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}

type composeCheckEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"` // ok | missing | stale
}

func newComposeCheckCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check compose targets for staleness",
		Long: `Compare the on-disk generated file for each compose target against the
freshly assembled content. Exits non-zero if any target is missing or stale.

Examples:
  dotd compose check
  dotd compose check --json
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
			entries := make([]composeCheckEntry, 0)
			for _, gen := range res.Generated {
				if gen.Path == "" {
					continue
				}
				status := "ok"
				existing, readErr := os.ReadFile(gen.Path)
				if errors.Is(readErr, fs.ErrNotExist) {
					status = "missing"
					hasStale = true
				} else if readErr != nil {
					return fmt.Errorf("compose check: read %s: %w", gen.Path, readErr)
				} else if !bytes.Equal(existing, gen.Content) {
					status = "stale"
					hasStale = true
				}
				entries = append(entries, composeCheckEntry{Path: gen.Path, Status: status})
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			for _, e := range entries {
				switch e.Status {
				case "missing":
					ui.Missingf(errOut, "%s", filepath.Base(e.Path))
				case "stale":
					ui.Wrongf(errOut, "%s", filepath.Base(e.Path))
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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}

