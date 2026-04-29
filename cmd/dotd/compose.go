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
				return runComposeCheck(cfg)
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
		LinkRoot:     cfg.linkRoot,
		DryRun:       cfg.dryRun,
	}
	synthetic, err := composer.Apply(nodes.Compose(), opts)
	if err != nil {
		return err
	}
	if cfg.dryRun {
		for _, n := range synthetic {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s\n",
				ui.Header("compose:"), n.LogicalName, ui.Arrow("→"), n.Path)
		}
		return nil
	}
	for _, n := range synthetic {
		cfg.log.Debugf("%s %s %s %s", ui.Header("compose:"), n.LogicalName, ui.Arrow("→"), n.Path)
	}
	cfg.log.Infof("%s %d generated", ui.Header("compose:"), len(synthetic))
	return nil
}

func runComposeCheck(cfg *config) error {
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
		LinkRoot:     cfg.linkRoot,
	})
	if err != nil {
		return err
	}

	var ok, missing, stale int
	for _, s := range statuses {
		switch s.State {
		case composer.StateOK:
			ok++
			cfg.log.Debug(s.OutputPath, "state", "ok")
		case composer.StateMissing:
			missing++
			cfg.log.Warn(s.OutputPath, "state", "missing")
		case composer.StateStale:
			stale++
			cfg.log.Warn(s.OutputPath, "state", "stale")
		}
	}
	cfg.log.Infof("%s %d ok, %d missing, %d stale", ui.Header("compose:"), ok, missing, stale)
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
		if cfg.verbose() {
			for _, f := range s.Fragments {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f.Path)
			}
		}
	}
	return nil
}
