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

type composeListEntry struct {
	Path string `json:"path"`
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
  dotd compose list --json | jq -r '.[].path'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}
			entries := make([]composeListEntry, 0)
			for _, n := range ordered {
				if !n.HasCompose() {
					continue
				}
				entries = append(entries, composeListEntry{Path: pipeline.ComposeFileName(n.Path)})
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), entries)
			}
			for _, e := range entries {
				fmt.Fprintln(cmd.OutOrStdout(), e.Path)
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}

// Compose check status values — one definition shared by the JSON contract
// (composeCheckEntry.Status) and the text renderer's switch.
const (
	composeStatusOK      = "ok"
	composeStatusMissing = "missing"
	composeStatusStale   = "stale"
)

type composeCheckEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"` // composeStatusOK | composeStatusMissing | composeStatusStale
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
			actOpts := buildActOptions(cfg, true)
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
				status := composeStatusOK
				existing, readErr := os.ReadFile(gen.Path)
				if errors.Is(readErr, fs.ErrNotExist) {
					status = composeStatusMissing
					hasStale = true
				} else if readErr != nil {
					return fmt.Errorf("compose check: read %s: %w", gen.Path, readErr)
				} else if !bytes.Equal(existing, gen.Content) {
					status = composeStatusStale
					hasStale = true
				}
				entries = append(entries, composeCheckEntry{Path: gen.Path, Status: status})
			}

			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), entries); err != nil {
					return err
				}
			} else {
				for _, e := range entries {
					switch e.Status {
					case composeStatusMissing:
						ui.Missingf(errOut, "%s", filepath.Base(e.Path))
					case composeStatusStale:
						ui.Wrongf(errOut, "%s", filepath.Base(e.Path))
					}
				}
			}
			if hasStale {
				return &hintError{
					err:  errors.New("compose check: generated files stale or missing"),
					hint: "run 'dotd apply' to update",
				}
			}
			if !jsonOutput {
				// Whole report on one channel (stderr): per-file lines above
				// and this summary — stdout stays clean for --json/pipes.
				ui.OKf(errOut, "compose check: all targets up-to-date")
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}
