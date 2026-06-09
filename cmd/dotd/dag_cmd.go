package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDagCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Inspect the dotfile dependency graph",
		Long: `Inspect the dependency graph built from @after annotations.

Nodes are ordered by topological sort: any node listed in another node's
@after annotation comes first. Alphabetical order breaks ties.`,
	}
	cmd.AddCommand(newDagCheckCmd(cfg))
	return cmd
}

func newDagCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Print nodes in dependency order",
		Long: `Print the active nodes in the order they would be processed by 'dotd apply'.

Inactive nodes (filtered out by @when predicates) are excluded.

Examples:
  dotd dag check
  dotd dag check --env os=macos`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}
			for i, n := range ordered {
				fmt.Fprintf(cmd.OutOrStdout(), "%3d  %s\n", i+1, n.LogicalName)
				cfg.log.Debugf("node %d: %s (when=%q)", i+1, n.LogicalName, n.EffectiveWhen)
			}
			return nil
		},
	}
}
