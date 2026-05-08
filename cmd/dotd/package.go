package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPackageCmd(_ *config) *cobra.Command {
	return &cobra.Command{
		Use:   "package",
		Short: "Package management — filtered by active predicates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("dotd package: not yet migrated to v2 pipeline")
		},
	}
}
