package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newEnvCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment resolution — inspect and modify env.yaml",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Display all resolved env key-value pairs",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runEnvShow(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "get <key>",
			Short: "Get a specific env key",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runEnvGet(cmd, cfg, args[0])
			},
		},
		&cobra.Command{
			Use:   "set <key=value>",
			Short: "Set a key in env.yaml",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runEnvSet(cmd, cfg, args[0])
			},
		},
	)
	return cmd
}

func runEnvShow(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(resolved))
	for k := range resolved {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", ui.Key(k), resolved[k])
	}
	return nil
}

func runEnvGet(cmd *cobra.Command, cfg *config, key string) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	val, ok := resolved[key]
	if !ok {
		return fmt.Errorf("key %q not found in resolved environment", key)
	}
	fmt.Fprintln(cmd.OutOrStdout(), val)
	return nil
}

func runEnvSet(cmd *cobra.Command, cfg *config, kv string) error {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected key=value, got %q", kv)
	}
	key, val := parts[0], parts[1]

	ef, err := env.LoadEnvFileFromPath(cfg.envFile)
	if err != nil {
		return err
	}
	ef.Env[key] = val

	if cfg.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "would set %s=%s in %s\n", key, val, cfg.envFile)
		return nil
	}
	if err := env.SaveEnvFileToPath(cfg.envFile, ef); err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "set %s=%s in %s\n", key, val, cfg.envFile)
	}
	return nil
}
