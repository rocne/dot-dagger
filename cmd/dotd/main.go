// Command dotd manages shell script sourcing via DAG-ordered init.sh generation.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/initgen"
	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type config struct {
	dotfiles string
	envFile  string
	env      []string
	initFile string
	linkRoot string
	binDir   string
	dryRun   bool
	force    bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     "dotd",
		Short:   "Dotfiles script DAG — generates init.sh and applies conf/bin symlinks",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&cfg.dotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&cfg.envFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	apply := &cobra.Command{
		Use:   "apply",
		Short: "Full reconciliation — evaluate predicates, resolve DAG, symlink, generate init.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, cfg)
		},
	}
	apply.Flags().StringVar(&cfg.initFile, "init-file", defaultInitFile(), "path to write init.sh")
	apply.Flags().StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	apply.Flags().StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")

	check := &cobra.Command{
		Use:   "check",
		Short: "Full status and validation — environment health, filesystem drift, errors",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, cfg)
		},
	}
	check.Flags().StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	check.Flags().StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")

	install := &cobra.Command{
		Use:   "install",
		Short: "Set up dot-dagger — rc wiring and first-run configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "dotd install: not yet implemented")
			return nil
		},
	}

	envParent := &cobra.Command{
		Use:   "env",
		Short: "Inspect and modify env.yaml",
	}
	envParent.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "Show all resolved env key-value pairs",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runEnvList(cmd, cfg)
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

	root.AddCommand(apply, check, install, envParent)
	return root
}

// --- command implementations ---

func runApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}

	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}

	content := initgen.Generate(ordered, cfg.binDir)
	if cfg.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "# would write %s\n", cfg.initFile)
		fmt.Fprint(cmd.OutOrStdout(), string(content))
	} else {
		if err := initgen.WriteFile(cfg.initFile, content); err != nil {
			return err
		}
		if cfg.verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d scripts)\n", cfg.initFile, len(ordered))
		}
	}

	opts := linker.Options{
		RepoRoot: cfg.dotfiles,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.dotfiles)

	if cfg.dryRun {
		for _, l := range links {
			fmt.Fprintf(cmd.OutOrStdout(), "# symlink %s → %s (state: %s)\n", l.Src, l.Dst, l.State)
		}
		return nil
	}

	if err := linker.Apply(links, cfg.force); err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "applied %d symlinks\n", len(links))
	}
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}

	if cfg.verbose {
		fmt.Fprintln(cmd.OutOrStdout(), "=== environment ===")
		for _, k := range sortedKeys(resolved) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s=%s\n", k, resolved[k])
		}
	}

	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "scripts: %d active, DAG OK\n", len(ordered))

	opts := linker.Options{
		RepoRoot: cfg.dotfiles,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.dotfiles)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
		case linker.StateMissing:
			missing++
			fmt.Fprintf(cmd.OutOrStdout(), "  missing: %s\n", l.Dst)
		case linker.StateWrongTarget:
			wrong++
			fmt.Fprintf(cmd.OutOrStdout(), "  wrong-target: %s\n", l.Dst)
		case linker.StateConflict:
			conflict++
			fmt.Fprintf(cmd.OutOrStdout(), "  conflict: %s\n", l.Dst)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "symlinks: %d ok, %d missing, %d wrong-target, %d conflict\n",
		ok, missing, wrong, conflict)
	return nil
}

func runEnvList(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	for _, k := range sortedKeys(resolved) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, resolved[k])
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

// --- helpers ---

func resolveEnv(cfg *config) (map[string]string, error) {
	ef, err := env.LoadEnvFileFromPath(cfg.envFile)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range cfg.env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}
	r := env.NewResolver()
	return r.Resolve(overrides)
}

func buildFileSet(cfg *config, resolved map[string]string) (*fileset.Set, error) {
	walked, err := walk.Walk(cfg.dotfiles)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.dotfiles, err)
	}
	return fileset.Build(walked, resolved, nil)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func defaultDotfiles() string {
	if d, ok := os.LookupEnv("DOTFILES"); ok {
		return d
	}
	dir, _ := os.Getwd()
	return dir
}

func defaultEnvFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dot-dagger", "env.yaml")
}

func defaultInitFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dot-dagger", "init.sh")
}
