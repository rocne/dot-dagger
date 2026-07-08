package main

import (
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/ui"
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
	Reason      string   `json:"reason,omitempty"`
}

func runList(cmd *cobra.Command, cfg *config, showInactive, jsonOutput bool) error {
	ordered, unmet, err := cfg.walkOrderedWithRequires(cmd)
	if err != nil {
		return err
	}

	// A node with an unmet @require is excluded from the active set here
	// (mirrors an unmet @when) rather than erroring — 'list' stays usable for
	// diagnosis; 'apply' is the one that hard-errors (see runApply).
	unmetPkgs := map[string][]string{} // LogicalName -> unmet package names
	for _, u := range unmet {
		unmetPkgs[u.Node] = append(unmetPkgs[u.Node], u.Package)
	}

	errOut := cmd.ErrOrStderr()
	var trulyActive []pipeline.RawNode
	reasonByPath := map[string]string{}
	for _, n := range ordered {
		pkgs, isUnmet := unmetPkgs[n.LogicalName]
		if !isUnmet {
			trulyActive = append(trulyActive, n)
			continue
		}
		reasonByPath[n.Path] = requireReason(pkgs)
		ui.Warnf(errOut, "%s: %s", n.LogicalName, requireReason(pkgs))
	}

	// Build active set for tagging (used when showInactive=true).
	activeSet := map[string]bool{}
	for _, n := range trulyActive {
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
			entries = append(entries, toEntry(n, activeSet[n.Path], reasonByPath[n.Path]))
		}
	} else {
		for _, n := range trulyActive {
			entries = append(entries, toEntry(n, true, ""))
		}
	}

	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), entries)
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
		line := fmt.Sprintf("%s%-40s  %-20s  %s",
			status, e.LogicalName, strings.Join(e.Actions, ","), e.Path)
		if e.Reason != "" {
			line += "  (" + e.Reason + ")"
		}
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return nil
}

func toEntry(n pipeline.RawNode, active bool, reason string) listEntry {
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
		Reason:      reason,
	}
}
