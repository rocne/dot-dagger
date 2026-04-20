package main

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/composer"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newComposeCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Compose targets — assemble fragment sets into generated files",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "apply",
			Short: "Generate all composed files",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runComposeApply(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Validate compose targets — report stale or missing generated files",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runComposeCheck(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all compose targets and their active fragments",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runComposeList(cmd, cfg)
			},
		},
	)
	return cmd
}

func runComposeApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	opts := composer.Options{
		GeneratedDir: cfg.generatedDir,
		DryRun:       cfg.dryRun,
	}
	synthetic, err := composer.Apply(nodes.Compose(), opts)
	if err != nil {
		return err
	}
	if cfg.verbose || cfg.dryRun {
		for _, n := range synthetic {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s\n",
				ui.Header("compose:"), n.LogicalName, ui.Arrow("→"), n.Path)
		}
	}
	if !cfg.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d generated\n", ui.Header("compose:"), len(synthetic))
	}
	return nil
}

func runComposeCheck(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	statuses, err := composer.Check(nodes.Compose(), composer.Options{
		GeneratedDir: cfg.generatedDir,
	})
	if err != nil {
		return err
	}

	var ok, missing, stale int
	for _, s := range statuses {
		switch s.State {
		case composer.StateOK:
			ok++
			if cfg.verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.OK("ok"), s.OutputPath)
			}
		case composer.StateMissing:
			missing++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Missing("missing"), s.OutputPath)
		case composer.StateStale:
			stale++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Wrong("stale"), s.OutputPath)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d ok, %d missing, %d stale\n",
		ui.Header("compose:"), ok, missing, stale)
	return nil
}

func runComposeList(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	summaries, err := composer.List(nodes.Compose(), cfg.generatedDir)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s no active compose targets\n", ui.Header("compose:"))
		return nil
	}
	for _, s := range summaries {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s (%d fragments)\n",
			ui.Header("compose:"), s.Dir, ui.Arrow("→"), s.OutputName, len(s.Fragments))
		if cfg.verbose {
			for _, f := range s.Fragments {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f.Path)
			}
		}
	}
	return nil
}
