package main

import (
	"encoding/json"
	"fmt"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and modify tool configuration",
		Long: `Read and write entries in config.yaml.

Stored values are path defaults for dot-dagger itself: where the dotfiles
repo lives, where bin scripts are linked, where generated files go, and
which directory ~ expands to in link destinations.`,
	}
	cmd.AddCommand(
		newConfigShowCmd(cfg),
		newConfigGetCmd(cfg),
		newConfigSetCmd(cfg),
		newConfigEditCmd(cfg),
	)
	return cmd
}

// configKeyArgs accepts exactly nArgs arguments. On error, the hint lists
// the valid config keys.
func configKeyArgs(nArgs int, usage string) cobra.PositionalArgs {
	return keyArgs(nArgs, usage, "valid keys: "+strings.Join(dotcfg.Keys, ", "))
}

// configKeyError wraps an unknown-key error from dotcfg.Get/Set with a hint
// listing the valid keys.
func configKeyError(err error) error {
	return &hintError{
		err:  err,
		hint: "valid keys: " + strings.Join(dotcfg.Keys, ", "),
	}
}

type configEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func newConfigShowCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display all config key=value pairs",
		Long: `Display every config key and its current value.

Examples:
  dotd config show
  dotd config show --json | jq '.[] | select(.key=="dotfiles")'
  dotd config show | grep dotfiles`,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := dotcfg.Load(cfg.configPath)
			if err != nil {
				return err
			}
			if jsonOutput {
				entries := make([]configEntry, 0, len(dotcfg.Keys))
				for _, k := range dotcfg.Keys {
					val, _ := toolCfg.Get(k)
					entries = append(entries, configEntry{Key: k, Value: val})
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, k := range dotcfg.Keys {
				val, _ := toolCfg.Get(k)
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, val)
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}

func newConfigGetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single config value",
		Long: `Print the value of a single config key to stdout.

Examples:
  dotd config get dotfiles
  dotd config get link_root`,
		Args: configKeyArgs(1, "usage: dotd config get <key>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := dotcfg.Load(cfg.configPath)
			if err != nil {
				return err
			}
			val, err := toolCfg.Get(args[0])
			if err != nil {
				return configKeyError(err)
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
		Long: `Write a value to a config key. Persists to config.yaml.

Examples:
  dotd config set dotfiles ~/dotfiles
  dotd config set link_root /home/me`,
		Args: configKeyArgs(2, "usage: dotd config set <key> <value>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolCfg, err := dotcfg.Load(cfg.configPath)
			if err != nil {
				return err
			}
			if err := toolCfg.Set(args[0], args[1]); err != nil {
				return configKeyError(err)
			}
			return dotcfg.Save(cfg.configPath, toolCfg)
		},
	}
}

func newConfigEditCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config.yaml in $EDITOR",
		Long: `Open config.yaml in $EDITOR. Falls back to vi if $EDITOR is unset.

Examples:
  dotd config edit
  EDITOR=nano dotd config edit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchEditor(cfg.configPath)
		},
	}
}
