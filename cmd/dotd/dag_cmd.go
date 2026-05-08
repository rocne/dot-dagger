package main

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newDagCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dag",
		Short:  "Inspect the dotfile dependency graph",
		Hidden: true,
	}
	cmd.AddCommand(newDagCheckCmd(cfg))
	return cmd
}

func newDagCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Print nodes in dependency order",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, err := pipeline.Walk(cfg.files)
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
			for i, n := range ordered {
				fmt.Fprintf(cmd.OutOrStdout(), "%3d  %s\n", i+1, n.LogicalName)
				cfg.log.Debugf("node %d: %s (when=%q)", i+1, n.LogicalName, n.EffectiveWhen)
			}
			return nil
		},
	}
}
