package main

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/spf13/cobra"
)

type pathRow struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func newPathsCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Show where anchors and tool paths resolve on this machine",
		Long: `Print the resolved locations of every anchor token and tool-managed path.

Examples:
  dotd paths
  dotd paths --json | jq '.[] | select(.name=="$config")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows := []pathRow{
				{"home", cfg.home},
				{"$bin", cfg.binDir},
				{"$config", cfg.configDir},
				{"generated", cfg.generatedDir},
				{"init.sh", cfg.initFile},
				{"dotfiles", cfg.files},
				{ecosystem.ConfigFileName, cfg.configPath},
				{ecosystem.EnvFileName, cfg.envFile},
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), rows)
			}
			for _, r := range rows {
				// column width 11 = len("config.yaml"), the longest label; bump if a longer name is added
				fmt.Fprintf(cmd.OutOrStdout(), "%-11s %s\n", r.Name, r.Path)
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}
