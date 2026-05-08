package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newEnvCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Inspect and modify env.yaml",
	}
	cmd.AddCommand(
		newEnvShowCmd(cfg),
		newEnvGetCmd(cfg),
		newEnvSetCmd(cfg),
		newEnvDiffCmd(cfg),
		newEnvEditCmd(cfg),
	)
	return cmd
}

// resolvedEnvFull returns the fully resolved env map: built-ins < env.yaml < DOTD_* < --env flags.
func resolvedEnvFull(cfg *config) (map[string]string, error) {
	r := env.NewResolver()
	out, _ := r.Resolve(nil)
	fileAndShell, err := env.ResolveWithOverrides(cfg.envFile, cfg.env)
	if err != nil {
		return nil, err
	}
	for k, v := range fileAndShell {
		out[k] = v
	}
	return out, nil
}

func newEnvShowCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all resolved env key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvedEnvFull(cfg)
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(resolved))
			for k := range resolved {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, resolved[k])
			}
			return nil
		},
	}
}

func newEnvGetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single env key value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvedEnvFull(cfg)
			if err != nil {
				return err
			}
			val, ok := resolved[args[0]]
			if !ok {
				return fmt.Errorf("key %q not found", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

func newEnvSetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key=value>",
		Short: "Set a key in env.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := strings.IndexByte(args[0], '=')
			if idx < 0 {
				return fmt.Errorf("argument must be key=value, got %q", args[0])
			}
			key := args[0][:idx]
			val := args[0][idx+1:]
			if cfg.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would set %s=%s\n", key, val)
				return nil
			}
			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}
			raw[key] = val
			return env.Save(cfg.envFile, raw)
		},
	}
}

func newEnvDiffCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show env.yaml keys that differ from built-in detected values",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := env.NewResolver()
			detected, _ := r.Resolve(nil)

			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}

			var diffs []string
			for k, fileVal := range raw {
				if detectedVal, ok := detected[k]; ok && fileVal != detectedVal {
					diffs = append(diffs, fmt.Sprintf("%s: %s → %s", k, detectedVal, fileVal))
				}
			}

			if len(diffs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no overrides")
				return nil
			}
			sort.Strings(diffs)
			for _, d := range diffs {
				fmt.Fprintln(cmd.OutOrStdout(), d)
			}
			return nil
		},
	}
}

func newEnvEditCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open env.yaml in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, cfg.envFile)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}
