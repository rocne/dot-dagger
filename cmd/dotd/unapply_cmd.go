package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newUnapplyCmd(cfg *config) *cobra.Command {
	var yes bool
	var all bool
	cmd := &cobra.Command{
		Use:   "unapply",
		Short: "Remove symlinks created by 'dotd apply'",
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
	// Note: --all is also a persistent flag on the root command (for --help --all).
	// pflag's AddFlagSet skips existing flags, so this local flag wins during parsing
	// for the unapply subcommand. Root's SetHelpFunc reads cmd.Root().PersistentFlags()
	// directly — unaffected by this shadow.
	cmd.Flags().BoolVar(&all, "all", false, "remove all dotfiles symlinks regardless of @when predicates")
	return cmd
}

func runUnapply(cmd *cobra.Command, cfg *config, yes, all bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	// Collect the link plan.
	type linkPair struct{ src, dest string }
	var planned []linkPair

	if all {
		// Walk all nodes (no predicate filter), get all link destinations.
		nodes, _, err := pipeline.Walk(cfg.files)
		if err != nil {
			return fmt.Errorf("walk %s: %w", cfg.files, err)
		}
		ordered, err := pipeline.Order(nodes)
		if err != nil {
			return fmt.Errorf("order: %w", err)
		}
		res, err := pipeline.Act(ordered, buildActOptions(cfg, true))
		if err != nil {
			return fmt.Errorf("act: %w", err)
		}
		for _, lnk := range res.Links {
			planned = append(planned, linkPair{src: lnk.Src, dest: lnk.Dest})
		}
	} else {
		prun, err := runPipeline(cfg, true)
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
	initShExists := fileExists(cfg.initFile)

	if len(toRemove) == 0 && !initShExists {
		fmt.Fprintf(out, "%s\n", ui.Skip("nothing to remove"))
		return nil
	}

	// Preview.
	if initShExists {
		fmt.Fprintf(out, "%s %d symlink(s) and init.sh:\n", ui.Header("Will remove"), len(toRemove))
	} else {
		fmt.Fprintf(out, "%s %d symlink(s):\n", ui.Header("Will remove"), len(toRemove))
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
	if !yes {
		fmt.Fprint(out, "\nProceed? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Fprintf(out, "%s\n", ui.Skip("cancelled"))
			return nil
		}
	}

	// Execute.
	for _, dest := range toRemove {
		if err := os.Remove(dest); err != nil {
			ui.Errf(out, "removing %s: %v", dest, err)
			continue
		}
		fmt.Fprintf(out, "%s %s\n", ui.OK("removed"), dest)
	}
	if initShExists {
		if err := os.Remove(cfg.initFile); err != nil {
			ui.Errf(out, "removing %s: %v", cfg.initFile, err)
		} else {
			fmt.Fprintf(out, "%s %s\n", ui.OK("removed"), cfg.initFile)
		}
	}

	return nil
}
