package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newEnvCmd(_ *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Inspect and modify env.yaml",
	}
	cmd.AddCommand(
		newEnvShowCmd(),
		newEnvGetCmd(),
		newEnvSetCmd(),
		newEnvEditCmd(),
	)
	return cmd
}

func envYamlPath() (string, error) {
	if p := os.Getenv("DOTD_ENV_FILE"); p != "" {
		return p, nil
	}
	return env.DefaultPath()
}

func resolvedEnv() (map[string]string, error) {
	path, err := envYamlPath()
	if err != nil {
		return nil, err
	}
	raw, err := env.Load(path)
	if err != nil {
		return nil, err
	}
	expanded, err := env.Expand(raw)
	if err != nil {
		return nil, err
	}
	shellVars := env.ShellVars(os.Environ())
	return env.Resolve(map[string]string{}, shellVars, expanded), nil
}

func newEnvShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display all resolved env key=value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvedEnv()
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

func newEnvGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single env key value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvedEnv()
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

func newEnvSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a key in env.yaml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := envYamlPath()
			if err != nil {
				return err
			}
			raw, err := env.Load(path)
			if err != nil {
				return err
			}
			raw[args[0]] = args[1]
			return env.Save(path, raw)
		},
	}
}

func newEnvEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open env.yaml in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := envYamlPath()
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
