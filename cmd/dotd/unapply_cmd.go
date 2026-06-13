package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newUnapplyCmd(cfg *config) *cobra.Command {
	var yes bool
	var all bool
	cmd := &cobra.Command{
		Use:     "unapply",
		Aliases: []string{"remove"},
		Short:   "Remove symlinks created by 'dotd apply'",
		Long: `Remove symlinks that were created by 'dotd apply'.

Re-runs the pipeline to determine the expected link plan, then removes each
symlink whose destination points to the expected source file.

Use --all to remove all symlinks pointing into the dotfiles repo regardless
of @when predicates — useful when you applied with --env flags.

Examples:
  dotd unapply
  dotd unapply --dry-run
  dotd unapply --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnapply(cmd, cfg, yes, all)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&all, "all", false, "remove all dotfiles symlinks regardless of @when predicates")
	return cmd
}

func runUnapply(cmd *cobra.Command, cfg *config, yes, all bool) error {
	out := cmd.OutOrStdout()

	// Collect the link plan.
	type linkPair struct{ src, dest string }
	var planned []linkPair

	if all {
		// Walk all nodes (no predicate filter), get all link destinations.
		nodes, _, err := pipeline.Walk(cfg.files)
		if err != nil {
			return fmt.Errorf("unapply: walk %s: %w", cfg.files, err)
		}
		ordered, err := pipeline.Order(nodes)
		if err != nil {
			return fmt.Errorf("unapply: order: %w", err)
		}
		actOpts := buildActOptions(cfg, true)
		res, err := pipeline.Act(ordered, actOpts)
		if err != nil {
			return fmt.Errorf("unapply: %w", err)
		}
		for _, lnk := range res.Links {
			planned = append(planned, linkPair{src: lnk.Src, dest: lnk.Dest})
		}
	} else {
		prun, err := runPipeline(cmd, cfg, true)
		if err != nil {
			return err
		}
		for _, lnk := range prun.result.Links {
			planned = append(planned, linkPair{src: lnk.Src, dest: lnk.Dest})
		}
	}

	// Determine which planned links are currently active symlinks.
	dotfilesRoot := cfg.files + string(filepath.Separator)
	var toRemove []string
	for _, lnk := range planned {
		target, err := os.Readlink(lnk.dest)
		if err != nil {
			continue // not a symlink or missing
		}
		if all {
			// Remove if symlink points into dotfiles repo.
			if strings.HasPrefix(target, dotfilesRoot) || target == cfg.files {
				toRemove = append(toRemove, lnk.dest)
			}
		} else {
			// Remove only if target matches exactly.
			if target == lnk.src {
				toRemove = append(toRemove, lnk.dest)
			}
		}
	}

	// Check for init.sh.
	initShExists := fileutil.Exists(cfg.initFile)

	if len(toRemove) == 0 && !initShExists {
		ui.Skipf(out, "nothing to remove")
		return nil
	}

	// Preview.
	if initShExists {
		ui.Headerf(out, "Will remove %s and init.sh:", plural(len(toRemove), "symlink"))
	} else {
		ui.Headerf(out, "Will remove %s:", plural(len(toRemove), "symlink"))
	}
	for _, dest := range toRemove {
		fmt.Fprintf(out, "  %s\n", dest)
	}
	if initShExists {
		fmt.Fprintf(out, "  %s\n", cfg.initFile)
	}

	// Dry-run stops here.
	if cfg.dryRun {
		return nil
	}

	// Confirmation.
	if !yes && !promptConfirm(out, cmd.InOrStdin()) {
		return nil
	}

	// Execute. Failures go to stderr and bubble up as a non-zero exit so
	// scripts can tell the difference between "nothing removed" and "tried
	// to remove but couldn't".
	errOut := cmd.ErrOrStderr()
	var failures int
	for _, dest := range toRemove {
		if err := os.Remove(dest); err != nil {
			ui.Errf(errOut, "removing %s: %v", dest, err)
			failures++
			continue
		}
		ui.OKf(out, "removed %s", dest)
	}
	if initShExists {
		if err := os.Remove(cfg.initFile); err != nil {
			ui.Errf(errOut, "removing %s: %v", cfg.initFile, err)
			failures++
		} else {
			ui.OKf(out, "removed %s", cfg.initFile)
		}
	}

	if failures > 0 {
		return fmt.Errorf("unapply: %d of %d targets failed to remove", failures, len(toRemove)+boolToInt(initShExists))
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
