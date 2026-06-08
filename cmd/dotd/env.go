package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newEnvCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: fmt.Sprintf("Inspect and modify %s", ecosystem.EnvFileName),
	}
	cmd.AddCommand(
		newEnvShowCmd(cfg),
		newEnvGetCmd(cfg),
		newEnvSetCmd(cfg),
		newEnvEditCmd(cfg),
		newEnvDiffCmd(cfg),
	)
	return cmd
}

func envYamlPath(cfg *config) string {
	return cfg.envFile
}

func newEnvShowCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all resolved env key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return err
			}
			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(resolved))
			for k := range resolved {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rawVal := raw[k]
				if strings.HasPrefix(rawVal, "$(") && strings.HasSuffix(rawVal, ")") {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\t[%s]\n", k, resolved[k], rawVal)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, resolved[k])
				}
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
			resolved, err := resolveEnv(cfg)
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
		Use:   "set <key> <value>",
		Short: fmt.Sprintf("Set a key in %s", ecosystem.EnvFileName),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := envYamlPath(cfg)
			raw, err := env.Load(path)
			if err != nil {
				return err
			}
			raw[args[0]] = args[1]
			return env.Save(path, raw)
		},
	}
}

func newEnvEditCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: fmt.Sprintf("Open %s in $EDITOR", ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchEditor(envYamlPath(cfg))
		},
	}
}

func newEnvDiffCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: fmt.Sprintf("Show %s keys that override shell-detected values", ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Intentionally compares env.yaml values against DOTD_* shell vars,
			// not the final resolved env. --env CLI overrides are not included —
			// diff shows what the file contributes, not what the invocation overrides.
			raw, err := env.Load(cfg.envFile)
			if err != nil {
				return err
			}
			expanded, err := env.Expand(raw)
			if err != nil {
				return err
			}
			shellVars := env.ShellVars(os.Environ())

			keys := make([]string, 0, len(expanded))
			for k := range expanded {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			hasAny := false
			for _, k := range keys {
				envVal := expanded[k]
				shellVal, inShell := shellVars[k]
				if inShell && envVal == shellVal {
					continue
				}
				if inShell {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %q (shell) → %q (%s)\n", k, shellVal, envVal, ecosystem.EnvFileName)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: (unset) → %q (%s)\n", k, envVal, ecosystem.EnvFileName)
				}
				hasAny = true
			}
			if !hasAny {
				fmt.Fprintf(cmd.OutOrStdout(), "no overrides — %s values match shell\n", ecosystem.EnvFileName)
			}
			return nil
		},
	}
}
