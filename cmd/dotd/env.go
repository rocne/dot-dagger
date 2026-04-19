package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

// envDiffEntry records a single key's detected vs effective value.
type envDiffEntry struct {
	key      string
	detected string
	override string
}

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
		&cobra.Command{
			Use:   "diff",
			Short: "Show keys where env.yaml overrides auto-detected values",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runEnvDiff(cmd, cfg)
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

	if !cfg.verbose {
		for _, k := range keys {
			fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", ui.Key(k), resolved[k])
		}
		return nil
	}

	// Verbose: show source of each value.
	detected, _ := env.NewResolver().Resolve(nil)
	ef, _ := env.LoadEnvFileFromPath(cfg.envFile)
	cliOverrides := make(map[string]string)
	for _, kv := range cfg.env {
		if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
			cliOverrides[parts[0]] = parts[1]
		}
	}

	source := func(k string) string {
		if _, ok := cliOverrides[k]; ok {
			return "--env flag"
		}
		if _, ok := ef.Env[k]; ok {
			return "env.yaml"
		}
		if _, ok := detected[k]; ok {
			return "detected"
		}
		return "unknown"
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(w, "%s=%s\t(%s)\n", ui.Key(k), resolved[k], source(k))
	}
	return w.Flush()
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

func runEnvDiff(cmd *cobra.Command, cfg *config) error {
	// Load raw env.yaml overrides (no CLI flags — we want file-level overrides only).
	ef, err := env.LoadEnvFileFromPath(cfg.envFile)
	if err != nil {
		return err
	}

	// Detect without any overrides to get raw detected values.
	detected, err := env.NewResolver().Resolve(nil)
	if err != nil {
		return err
	}

	// Collect keys where env.yaml diverges from detected.
	var diffs []envDiffEntry
	for k, override := range ef.Env {
		det := detected[k]
		if det != override {
			diffs = append(diffs, envDiffEntry{key: k, detected: det, override: override})
		}
	}
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].key < diffs[j].key })

	if len(diffs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no overrides — env.yaml matches auto-detected values")
		return nil
	}
	for _, d := range diffs {
		if d.detected == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  (not detected) → %s\n", ui.Key(d.key), d.override)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s → %s\n", ui.Key(d.key), d.detected, d.override)
		}
	}
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
