// Command dote resolves and displays the environment used by the dotr suite.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type config struct {
	envFile string
	env     []string
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     "dote",
		Short:   "Environment resolution for the dotr suite",
		Version: version,
	}

	ui.SetupCobraColors(root)

	show := &cobra.Command{
		Use:   "show",
		Short: "Display the resolved environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, cfg)
		},
	}
	show.Flags().StringVar(&cfg.envFile, "env-file", defaultEnvFile(), "path to env.yaml")
	show.Flags().StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")

	root.AddCommand(show)
	return root
}

func runShow(cmd *cobra.Command, cfg *config) error {
	ef, err := env.LoadEnvFileFromPath(cfg.envFile)
	if err != nil {
		return err
	}

	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range cfg.env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}

	r := env.NewResolver()
	resolved, err := r.Resolve(overrides)

	printEnv(cmd, resolved)

	return err
}

func printEnv(cmd *cobra.Command, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", ui.Key(k), m[k])
	}
}

func defaultEnvFile() string {
	p, err := ecosystem.DefaultEnvFile()
	if err != nil {
		panic(err)
	}
	return p
}
