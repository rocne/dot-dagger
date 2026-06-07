package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDagCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Inspect the dotfile dependency graph",
	}
	cmd.AddCommand(newDagCheckCmd(cfg))
	return cmd
}

func newDagCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Print nodes in dependency order",
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
