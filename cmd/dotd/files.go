package main

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

func newFilesCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Inspect the active file set",
	}

	var showAll bool

	list := &cobra.Command{
		Use:   "list",
		Short: "List files in the active set",
		Long: `List files in the active set for the current environment.

By default only active (predicate-passing) files are shown. Use --all to
include inactive and disabled files alongside their conditions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFilesList(cmd, cfg, showAll)
		},
	}
	list.Flags().BoolVar(&showAll, "all", false, "include inactive and disabled files")

	cmd.AddCommand(list)
	return cmd
}

func runFilesList(cmd *cobra.Command, cfg *config, showAll bool) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}

	walked, err := walk.Walk(cfg.files)
	if err != nil {
		return fmt.Errorf("walk %s: %w", cfg.files, err)
	}

	if !showAll {
		active, err := fileset.Build(walked, resolved, nil)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		for _, n := range active.Nodes {
			rel, _ := filepath.Rel(cfg.files, n.Path)
			fmt.Fprintf(w, "%s\t%s\t%s\n", kindLabel(n.Kind), n.LogicalName, rel)
		}
		return w.Flush()
	}

	// --all: show every walked node with its status.
	active, err := fileset.Build(walked, resolved, nil)
	if err != nil {
		return err
	}
	activePaths := make(map[string]bool, len(active.Nodes))
	for _, n := range active.Nodes {
		activePaths[n.Path] = true
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, n := range walked {
		rel, _ := filepath.Rel(cfg.files, n.Path)
		status := nodeStatus(n, activePaths)
		cond := ""
		if n.EffectiveWhen != "" {
			cond = "[" + n.EffectiveWhen + "]"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", status, kindLabel(n.Kind), n.LogicalName, rel, cond)
	}
	return w.Flush()
}

func kindLabel(k fileset.Kind) string {
	switch k {
	case fileset.KindScript:
		return "shellrc"
	case fileset.KindConf:
		return "conf"
	case fileset.KindBin:
		return "bin"
	default:
		return "other"
	}
}

func nodeStatus(n walk.Node, activePaths map[string]bool) string {
	if _, ok := annotation.First(n.Annotations, annotation.KeyDisable); ok {
		return "disabled"
	}
	if activePaths[n.Path] {
		return "active"
	}
	return "inactive"
}
