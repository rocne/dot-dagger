// Command dotl applies, checks, and removes symlinks for conf/ and bin/ nodes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
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
	Use:   "dotl",
	Short: "Dotfiles linker — symlinks conf/ and bin/ files into the system",
}

var (
	flagDotfiles string
	flagEnvFile  string
	flagEnv      []string
	flagLinkRoot string
	flagBinDir   string
	flagDryRun   bool
	flagForce    bool
	flagVerbose  bool
)

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagDotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&flagEnvFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&flagEnv, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&flagLinkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&flagBinDir, "bin-dir", "", "bin directory for bin/ files")
	pf.BoolVar(&flagDryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&flagForce, "force", false, "override safety checks")
	pf.BoolVar(&flagVerbose, "verbose", false, "detailed output")

	rootCmd.AddCommand(applyCmd, checkCmd, removeCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Plan and apply symlinks for active conf/ and bin/ nodes",
	RunE:  runApply,
}

func runApply(cmd *cobra.Command, args []string) error {
	nodes, err := buildFileSet()
	if err != nil {
		return err
	}

	links, err := planLinks(nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, flagDotfiles)

	if flagDryRun {
		for _, l := range links {
			if l.State != linker.StateOK {
				fmt.Printf("symlink %s → %s\n", l.Src, l.Dst)
			}
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
	Short: "Report symlink state without making changes",
	RunE:  runCheck,
}

func runCheck(cmd *cobra.Command, args []string) error {
	nodes, err := buildFileSet()
	if err != nil {
		return err
	}

	links, err := planLinks(nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, flagDotfiles)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
			if flagVerbose {
				fmt.Printf("  ok       %s\n", l.Dst)
			}
		case linker.StateMissing:
			missing++
			fmt.Printf("  missing  %s\n", l.Dst)
		case linker.StateWrongTarget:
			wrong++
			fmt.Printf("  wrong    %s → %s\n", l.Dst, l.Src)
		case linker.StateConflict:
			conflict++
			fmt.Printf("  conflict %s\n", l.Dst)
		}
	}
	fmt.Printf("%d ok, %d missing, %d wrong-target, %d conflict\n",
		ok, missing, wrong, conflict)
	return nil
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove owned symlinks",
	RunE:  runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	nodes, err := buildFileSet()
	if err != nil {
		return err
	}

	links, err := planLinks(nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, flagDotfiles)

	if flagDryRun {
		for _, l := range links {
			if l.Owned {
				fmt.Printf("remove %s\n", l.Dst)
			}
		}
		return nil
	}

	if err := linker.Remove(links); err != nil {
		return err
	}
	if flagVerbose {
		var removed int
		for _, l := range links {
			if l.Owned {
				removed++
			}
		}
		fmt.Printf("removed %d symlinks\n", removed)
	}
	return nil
}

// --- helpers ---

func buildFileSet() (*fileset.Set, error) {
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
	resolved, err := r.Resolve(overrides)
	if err != nil {
		return nil, err
	}

	walked, err := walk.Walk(flagDotfiles)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", flagDotfiles, err)
	}
	return fileset.Build(walked, resolved, nil)
}

func planLinks(nodes *fileset.Set) ([]linker.Link, error) {
	opts := linker.Options{
		RepoRoot: flagDotfiles,
		LinkRoot: flagLinkRoot,
		BinDir:   flagBinDir,
	}
	return linker.Plan(nodes.Nodes, opts)
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
