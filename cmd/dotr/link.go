package main

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newLinkCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Symlink management — predicate-filtered (see dotl for unconditional)",
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
				return runLinkCheck(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "remove",
			Short: "Remove owned symlinks",
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
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d symlinks\n", ui.OK("applied"), len(links))
	}
	return nil
}

func runLinkCheck(cmd *cobra.Command, cfg *config) error {
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

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
			if cfg.verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.OK("ok"), l.Dst)
			}
		case linker.StateMissing:
			missing++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Missing("missing"), l.Dst)
		case linker.StateWrongTarget:
			wrong++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s %s %s\n", ui.Wrong("wrong"), l.Dst, ui.Arrow("→"), l.Src)
		case linker.StateConflict:
			conflict++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Conflict("conflict"), l.Dst)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d ok, %d missing, %d wrong-target, %d conflict\n",
		ui.Header("symlinks:"), ok, missing, wrong, conflict)
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
		for _, l := range links {
			if l.Owned {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", ui.Wrong("remove"), l.Dst)
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d symlinks\n", ui.Missing("removed"), removed)
	}
	return nil
}
