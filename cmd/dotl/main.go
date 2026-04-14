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
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type config struct {
	dotfiles string
	envFile  string
	env      []string
	linkRoot string
	binDir   string
	dryRun   bool
	force    bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:   "dotl",
		Short: "Dotfiles linker — symlinks conf/ and bin/ files into the system",
	}

	pf := root.PersistentFlags()
	pf.StringVar(&cfg.dotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&cfg.envFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	root.AddCommand(
		&cobra.Command{
			Use:   "apply",
			Short: "Plan and apply symlinks for active conf/ and bin/ nodes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runApply(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Report symlink state without making changes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "remove",
			Short: "Remove owned symlinks",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runRemove(cmd, cfg)
			},
		},
	)
	return root
}

func runApply(cmd *cobra.Command, cfg *config) error {
	nodes, err := buildFileSet(cfg)
	if err != nil {
		return err
	}

	links, err := planLinks(cfg, nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, cfg.dotfiles)

	if cfg.dryRun {
		for _, l := range links {
			if l.State != linker.StateOK {
				fmt.Fprintf(cmd.OutOrStdout(), "symlink %s → %s\n", l.Src, l.Dst)
			}
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
	nodes, err := buildFileSet(cfg)
	if err != nil {
		return err
	}

	links, err := planLinks(cfg, nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, cfg.dotfiles)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
			if cfg.verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "  ok       %s\n", l.Dst)
			}
		case linker.StateMissing:
			missing++
			fmt.Fprintf(cmd.OutOrStdout(), "  missing  %s\n", l.Dst)
		case linker.StateWrongTarget:
			wrong++
			fmt.Fprintf(cmd.OutOrStdout(), "  wrong    %s → %s\n", l.Dst, l.Src)
		case linker.StateConflict:
			conflict++
			fmt.Fprintf(cmd.OutOrStdout(), "  conflict %s\n", l.Dst)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%d ok, %d missing, %d wrong-target, %d conflict\n",
		ok, missing, wrong, conflict)
	return nil
}

func runRemove(cmd *cobra.Command, cfg *config) error {
	nodes, err := buildFileSet(cfg)
	if err != nil {
		return err
	}

	links, err := planLinks(cfg, nodes)
	if err != nil {
		return err
	}
	links = linker.Check(links, cfg.dotfiles)

	if cfg.dryRun {
		for _, l := range links {
			if l.Owned {
				fmt.Fprintf(cmd.OutOrStdout(), "remove %s\n", l.Dst)
			}
		}
		return nil
	}

	if err := linker.Remove(links); err != nil {
		return err
	}
	if cfg.verbose {
		var removed int
		for _, l := range links {
			if l.Owned {
				removed++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "removed %d symlinks\n", removed)
	}
	return nil
}

// --- helpers ---

func buildFileSet(cfg *config) (*fileset.Set, error) {
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
	resolved, err := r.Resolve(overrides)
	if err != nil {
		return nil, err
	}

	walked, err := walk.Walk(cfg.dotfiles)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.dotfiles, err)
	}
	return fileset.Build(walked, resolved, nil)
}

func planLinks(cfg *config, nodes *fileset.Set) ([]linker.Link, error) {
	opts := linker.Options{
		RepoRoot: cfg.dotfiles,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
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
