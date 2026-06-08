package main

import (
	"fmt"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and modify tool configuration",
	}
	cmd.AddCommand(
		newConfigShowCmd(cfg),
		newConfigGetCmd(cfg),
		newConfigSetCmd(cfg),
		newConfigEditCmd(cfg),
	)
	return cmd
}

func loadConfig(configPath string) (*dotcfg.Config, error) {
	return dotcfg.Load(configPath)
}

func newConfigShowCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all config key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := loadConfig(cfg.configPath)
			if err != nil {
				return err
			}
			for _, k := range dotcfg.Keys {
				val, _ := toolCfg.Get(k)
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, val)
			}
			return nil
		},
	}
}

func newConfigGetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := loadConfig(cfg.configPath)
			if err != nil {
				return err
			}
			val, err := toolCfg.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

func newConfigSetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := loadConfig(cfg.configPath)
			if err != nil {
				return err
			}
			if err := toolCfg.Set(args[0], args[1]); err != nil {
				return err
			}
			return dotcfg.Save(cfg.configPath, toolCfg)
		},
	}
}

func newConfigEditCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config.yaml in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchEditor(cfg.configPath)
		},
	}
}
