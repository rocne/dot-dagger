// Command dotd manages shell script sourcing via DAG-ordered init.sh generation.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/initgen"
	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "dotd",
	Short: "Dotfiles script DAG — generates init.sh and applies conf/bin symlinks",
}

// Global flags shared by subcommands.
var (
	flagDotfiles string
	flagEnvFile  string
	flagEnv      []string
	flagDryRun   bool
	flagForce    bool
	flagVerbose  bool
)

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagDotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&flagEnvFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&flagEnv, "env", nil, "env override as key=value (repeatable)")
	pf.BoolVar(&flagDryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&flagForce, "force", false, "override safety checks")
	pf.BoolVar(&flagVerbose, "verbose", false, "detailed output")

	rootCmd.AddCommand(applyCmd, checkCmd, installCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Full reconciliation — evaluate predicates, resolve DAG, symlink, generate init.sh",
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVar(&flagInitFile, "init-file", defaultInitFile(), "path to write init.sh")
	applyCmd.Flags().StringVar(&flagLinkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	applyCmd.Flags().StringVar(&flagBinDir, "bin-dir", "", "bin directory for bin/ files")
}

var (
	flagInitFile string
	flagLinkRoot string
	flagBinDir   string
)

func runApply(cmd *cobra.Command, args []string) error {
	resolved, err := resolveEnv()
	if err != nil {
		return err
	}

	nodes, err := buildFileSet(resolved)
	if err != nil {
		return err
	}

	// DAG-order scripts, generate init.sh.
	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}

	content := initgen.Generate(ordered, flagBinDir)
	if flagDryRun {
		fmt.Printf("# would write %s\n", flagInitFile)
		fmt.Print(string(content))
	} else {
		if err := initgen.WriteFile(flagInitFile, content); err != nil {
			return err
		}
		if flagVerbose {
			fmt.Printf("wrote %s (%d scripts)\n", flagInitFile, len(ordered))
		}
	}

	// Plan and apply symlinks (conf + bin).
	opts := linker.Options{
		RepoRoot: flagDotfiles,
		LinkRoot: flagLinkRoot,
		BinDir:   flagBinDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, flagDotfiles)

	if flagDryRun {
		for _, l := range links {
			fmt.Printf("# symlink %s → %s (state: %s)\n", l.Src, l.Dst, l.State)
		}
		return nil
	}

	if err := linker.Apply(links, flagForce); err != nil {
		return err
	}
	if flagVerbose {
		fmt.Printf("applied %d symlinks\n", len(links))
	}
	return nil
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Full status and validation — environment health, filesystem drift, errors",
	RunE:  runCheck,
}

func init() {
	checkCmd.Flags().StringVar(&flagLinkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	checkCmd.Flags().StringVar(&flagBinDir, "bin-dir", "", "bin directory for bin/ files")
}

func runCheck(cmd *cobra.Command, args []string) error {
	resolved, err := resolveEnv()
	if err != nil {
		return err
	}

	if flagVerbose {
		fmt.Println("=== environment ===")
		for k, v := range resolved {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	nodes, err := buildFileSet(resolved)
	if err != nil {
		return err
	}

	// Check DAG validity.
	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Printf("scripts: %d active, DAG OK\n", len(ordered))

	// Check symlinks.
	opts := linker.Options{
		RepoRoot: flagDotfiles,
		LinkRoot: flagLinkRoot,
		BinDir:   flagBinDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, flagDotfiles)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
		case linker.StateMissing:
			missing++
			fmt.Printf("  missing: %s\n", l.Dst)
		case linker.StateWrongTarget:
			wrong++
			fmt.Printf("  wrong-target: %s\n", l.Dst)
		case linker.StateConflict:
			conflict++
			fmt.Printf("  conflict: %s\n", l.Dst)
		}
	}
	fmt.Printf("symlinks: %d ok, %d missing, %d wrong-target, %d conflict\n",
		ok, missing, wrong, conflict)
	return nil
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Set up dot-dagger — rc wiring and first-run configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "dotd install: not yet implemented")
		return nil
	},
}

// --- helpers ---

func resolveEnv() (map[string]string, error) {
	ef, err := env.LoadEnvFileFromPath(flagEnvFile)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range flagEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}
	r := env.NewResolver()
	return r.Resolve(overrides)
}

func buildFileSet(resolved map[string]string) (*fileset.Set, error) {
	walked, err := walk.Walk(flagDotfiles)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", flagDotfiles, err)
	}
	return fileset.Build(walked, resolved, nil)
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

