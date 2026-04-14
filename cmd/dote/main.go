// Command dote resolves and displays the environment used by the dotr suite.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "dote",
	Short: "Environment resolution for the dotr suite",
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the resolved environment",
	RunE:  runShow,
}

var (
	flagEnvFile string
	flagEnv     []string
)

func init() {
	showCmd.Flags().StringVar(&flagEnvFile, "env-file", defaultEnvFile(), "path to env.yaml")
	showCmd.Flags().StringArrayVar(&flagEnv, "env", nil, "env override as key=value (repeatable)")
	rootCmd.AddCommand(showCmd)
}

func defaultEnvFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dot-dagger", "env.yaml")
}

func runShow(cmd *cobra.Command, args []string) error {
	ef, err := env.LoadEnvFileFromPath(flagEnvFile)
	if err != nil {
		return err
	}

	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range flagEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}

	r := env.NewResolver()
	resolved, err := r.Resolve(overrides)

	// Print whatever resolved, even on partial error.
	printEnv(resolved)

	return err
}

func printEnv(m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s=%s\n", k, m[k])
	}
}
