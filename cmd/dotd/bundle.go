package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBundleCmd(_ *config) *cobra.Command {
	return &cobra.Command{
		Use:   "bundle <logical-name>",
		Short: "Bundle a node and its transitive @after dependencies into a single script",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("dotd bundle: not yet migrated to v2 pipeline")
		},
	}
}
