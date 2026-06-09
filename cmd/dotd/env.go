package main

import (
	"encoding/json"
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
		Long: fmt.Sprintf(`Read and write entries in %s.

%s is a flat YAML map of string keys to string values. Values starting with
$( ... ) are evaluated as shell expressions each time dotd runs. The
resolved env is used to evaluate @when predicates during the filter stage.

Resolution order (highest priority wins):
  1. --env flags        e.g. --env context=work
  2. DOTD_* shell vars  e.g. DOTD_CONTEXT=work
  3. %s values`, ecosystem.EnvFileName, ecosystem.EnvFileName, ecosystem.EnvFileName),
	}
	cmd.AddCommand(
		newEnvShowCmd(cfg),
		newEnvGetCmd(cfg),
		newEnvSetCmd(cfg),
		newEnvEditCmd(cfg),
		newEnvDiffCmd(cfg),
		newEnvPathCmd(cfg),
	)
	return cmd
}

// envKeyArgs accepts exactly nArgs arguments. On error, the returned
// hintError points the user at 'dotd env show' for the dynamic key set.
func envKeyArgs(nArgs int, usage string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == nArgs {
			return nil
		}
		return &hintError{
			err:  fmt.Errorf("expected %s, got %d", plural(nArgs, "argument"), len(args)),
			hint: usage + " — run 'dotd env show' to list keys",
		}
	}
}

func envYamlPath(cfg *config) string {
	return cfg.envFile
}

type envShowEntry struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Expression string `json:"expression,omitempty"`
}

func newEnvShowCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display all resolved env key=value pairs",
		Long: fmt.Sprintf(`Display every key in the resolved env map and its value.

Shell expressions are shown alongside the resolved value:
  key=resolved-value  [$(shell-expression)]

The resolved set is the merge of %s, DOTD_* shell vars, and --env overrides.

Examples:
  dotd env show
  dotd env show --env context=work
  dotd env show --json | jq -r '.[] | select(.expression) | .key'
  dotd env show | grep '^os='`, ecosystem.EnvFileName),
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
			if jsonOutput {
				entries := make([]envShowEntry, 0, len(keys))
				for _, k := range keys {
					e := envShowEntry{Key: k, Value: resolved[k]}
					if rawVal := raw[k]; strings.HasPrefix(rawVal, "$(") && strings.HasSuffix(rawVal, ")") {
						e.Expression = rawVal
					}
					entries = append(entries, e)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}

func newEnvGetCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single env key value",
		Long: `Print the resolved value of a single env key to stdout.

Examples:
  dotd env get os
  dotd env get context`,
		Args: envKeyArgs(1, "usage: dotd env get <key>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return err
			}
			val, ok := resolved[args[0]]
			if !ok {
				return &hintError{
					err:  fmt.Errorf("key %q not found", args[0]),
					hint: "run 'dotd env show' to list available keys",
				}
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
		Long: fmt.Sprintf(`Set a key in %s.

To store a shell expression that evaluates at runtime, use single quotes
to prevent the shell from expanding it:

  dotd env set os '$(dotd get-os)'
  dotd env set hostname '$(hostname)'

Values stored as $(…) are evaluated each time dotd runs.`, ecosystem.EnvFileName),
		Args: envKeyArgs(2, "usage: dotd env set <key> <value>"),
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

func newEnvPathCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the path to env.yaml",
		Long: fmt.Sprintf(`Print the resolved path to %s to stdout.

Useful for piping into other tools — e.g. cat "$(dotd env path)" or
opening it in a different editor.

Examples:
  dotd env path
  cat "$(dotd env path)"`, ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), cfg.envFile)
			return nil
		},
	}
}

func newEnvEditCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: fmt.Sprintf("Open %s in $EDITOR", ecosystem.EnvFileName),
		Long: fmt.Sprintf(`Open %s in $EDITOR. Falls back to vi if $EDITOR is unset.

Examples:
  dotd env edit
  EDITOR=nano dotd env edit`, ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchEditor(envYamlPath(cfg))
		},
	}
}

type envDiffEntry struct {
	Key       string `json:"key"`
	EnvValue  string `json:"env_value"`
	ShellSet  bool   `json:"shell_set"`
	ShellVal  string `json:"shell_value,omitempty"`
}

func newEnvDiffCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: fmt.Sprintf("Show %s keys that override shell-detected values", ecosystem.EnvFileName),
		Long: fmt.Sprintf(`Compare %s keys against the matching DOTD_* shell vars.

Each output line shows a key whose %s value differs from the shell-detected
value (or has no shell counterpart). --env CLI overrides are intentionally
excluded — diff shows what the file contributes, not what the invocation
overrides at runtime.

Examples:
  dotd env diff
  dotd env diff --json
  dotd env diff && echo "env.yaml is in sync"`, ecosystem.EnvFileName, ecosystem.EnvFileName),
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

			if jsonOutput {
				entries := make([]envDiffEntry, 0)
				for _, k := range keys {
					envVal := expanded[k]
					shellVal, inShell := shellVars[k]
					if inShell && envVal == shellVal {
						continue
					}
					entries = append(entries, envDiffEntry{
						Key:      k,
						EnvValue: envVal,
						ShellSet: inShell,
						ShellVal: shellVal,
					})
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}
