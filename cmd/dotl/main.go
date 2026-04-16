// Command dotl applies, checks, and removes symlinks for conf/ and bin/ nodes.
// Standalone mode: unconditional — all files in conf/ and bin/ are linked,
// regardless of @when predicates. Use dotr for predicate-filtered linking.
package main

import (
	"fmt"
	"os"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/ui"
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
	files string
	linkRoot string
	binDir   string
	dryRun   bool
	force    bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     ecosystem.ToolL,
		Short:   "Dotfiles linker — symlinks conf/ and bin/ files into the system (unconditional)",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	ui.SetupCobraColors(root)

	root.AddCommand(
		&cobra.Command{
			Use:   "apply",
			Short: "Plan and apply symlinks for all conf/ and bin/ nodes",
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
	links = linker.Check(links, cfg.files)

	if cfg.dryRun {
		for _, l := range links {
			if l.State != linker.StateOK {
				fmt.Fprintf(cmd.OutOrStdout(), "symlink %s %s %s\n", l.Src, ui.Arrow("→"), l.Dst)
			}
		}
		return nil
	}

	if err := linker.Apply(links, cfg.force); err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d symlinks\n", ui.OK("applied"), len(links))
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
	links = linker.Check(links, cfg.files)
	linker.PrintCheckSummary(cmd.OutOrStdout(), links, cfg.verbose)
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
	links = linker.Check(links, cfg.files)

	if cfg.dryRun {
		linker.PrintRemovePlan(cmd.OutOrStdout(), links)
		return nil
	}

	if err := linker.Remove(links); err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d symlinks\n", ui.Missing("removed"), linker.CountOwned(links))
	}
	return nil
}

func buildFileSet(cfg *config) (*fileset.Set, error) {
	walked, err := walk.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	return fileset.BuildUnfiltered(walked), nil
}

func planLinks(cfg *config, nodes *fileset.Set) ([]linker.Link, error) {
	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	return linker.Plan(nodes.Nodes, opts)
}

func defaultDotfiles() string { return ecosystem.DefaultDotfiles() }

