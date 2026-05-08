package main

import (
	"fmt"
	"os"
	"os/exec"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and modify tool configuration",
	}
	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigEditCmd(),
	)
	return cmd
}

func configYamlPath() (string, error) {
	return dotcfg.DefaultPath()
}

func loadConfig() (*dotcfg.Config, string, error) {
	path, err := configYamlPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := dotcfg.Load(path)
	return cfg, path, err
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all config key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			keys := []string{"dotfiles", "bin_dir", "generated_dir", "link_root"}
			for _, k := range keys {
				val, _ := cfg.Get(k)
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, val)
			}
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			val, err := cfg.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.Set(args[0], args[1]); err != nil {
				return err
			}
			return dotcfg.Save(path, cfg)
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config.yaml in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configYamlPath()
			if err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}
