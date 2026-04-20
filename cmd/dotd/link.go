package main

import (
	"fmt"
	"io"

	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newLinkCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Symlink management — filtered by active predicates",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "apply",
			Short: "Plan and apply symlinks for active conf/ and bin/ nodes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runLinkApply(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Report symlink state without making changes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runLinkCheck(cfg, cmd.ErrOrStderr())
			},
		},
		&cobra.Command{
			Use:   "remove",
			Short: "Remove symlinks that point into the dotfiles repo",
			Long: `Remove symlinks that are owned by dotd — those whose target points into the
dotfiles repo. Symlinks pointing elsewhere (pre-existing or foreign) are never
touched, even if they share the same destination path.

Use --dry-run to preview what would be removed before committing.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runLinkRemove(cmd, cfg)
			},
		},
	)
	return cmd
}

func runLinkApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
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
	cfg.log.Infof("%s %d symlinks", ui.OK("applied"), len(links))
	return nil
}

func runLinkCheck(cfg *config, errW io.Writer) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.files)
	linker.PrintCheckSummary(errW, links, cfg.verbose())
	return nil
}

func runLinkRemove(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.files)

	if cfg.dryRun {
		linker.PrintRemovePlan(cmd.OutOrStdout(), links)
		return nil
	}
	if err := linker.Remove(links); err != nil {
		return err
	}
	cfg.log.Infof("%s %d symlinks", ui.Missing("removed"), linker.CountOwned(links))
	return nil
}
