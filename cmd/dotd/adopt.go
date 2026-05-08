package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAdoptCmd(_ *config) *cobra.Command {
	return &cobra.Command{
		Use:   "adopt <file>",
		Short: "Move a file into the dotfiles repo and replace it with a symlink",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("dotd adopt: not yet migrated to v2 pipeline")
		},
	}
}
