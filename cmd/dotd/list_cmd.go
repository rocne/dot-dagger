package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newListCmd(cfg *config) *cobra.Command {
	var (
		showInactive bool
		jsonOutput   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dotfile nodes and their status",
		Long: `List nodes discovered by the pipeline (env → walk → filter → order).

Default: shows active nodes only. Each line: <logical-name>  <actions>  <path>

Examples:
  dotd list
  dotd list --inactive           # show nodes filtered out by predicates
  dotd list --json               # machine-readable JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, cfg, showInactive, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&showInactive, "inactive", false, "show nodes filtered out by predicates")
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}

type listEntry struct {
	LogicalName string   `json:"logical_name"`
	Path        string   `json:"path"`
	Actions     []string `json:"actions"`
	When        string   `json:"when,omitempty"`
	Active      bool     `json:"active"`
}

func runList(cmd *cobra.Command, cfg *config, showInactive, jsonOutput bool) error {
	ordered, err := cfg.walkOrdered(cmd)
	if err != nil {
		return err
	}

	// Build active set for tagging (used when showInactive=true).
	activeSet := map[string]bool{}
	for _, n := range ordered {
		activeSet[n.Path] = true
	}

	var entries []listEntry
	if showInactive {
		// Walk all nodes (active + inactive) so we can tag each.
		// This is the only path that needs the full unfiltered node list.
		nodes, _, err := pipeline.Walk(cfg.files)
		if err != nil {
			return fmt.Errorf("walk %s: %w", cfg.files, err)
		}
		for _, n := range nodes {
			entries = append(entries, toEntry(n, activeSet[n.Path]))
		}
	} else {
		for _, n := range ordered {
			entries = append(entries, toEntry(n, true))
		}
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(entries) == 0 {
		// Logged (stderr) so piped stdout stays empty for scripts.
		cfg.log.Infof("no nodes found in %s — is this a dotfiles repo?", cfg.files)
		return nil
	}

	for _, e := range entries {
		status := ""
		if showInactive {
			if e.Active {
				status = "active   "
			} else {
				status = "inactive "
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%-40s  %-20s  %s\n",
			status, e.LogicalName, strings.Join(e.Actions, ","), e.Path)
	}
	return nil
}

func toEntry(n pipeline.RawNode, active bool) listEntry {
	actions := make([]string, len(n.Actions))
	for i, a := range n.Actions {
		if a.Dest != "" {
			actions[i] = a.Type + "(" + a.Dest + ")"
		} else {
			actions[i] = a.Type
		}
	}
	return listEntry{
		LogicalName: n.LogicalName,
		Path:        n.Path,
		Actions:     actions,
		When:        n.EffectiveWhen,
		Active:      active,
	}
}
