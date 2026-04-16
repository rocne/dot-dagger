package main

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/initgen"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newDAGCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Shell init DAG — generate and validate init.sh script ordering",
	}

	apply := &cobra.Command{
		Use:   "apply",
		Short: "Resolve DAG and write init.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDAGApply(cmd, cfg)
		},
	}
	apply.Flags().StringVar(&cfg.initFile, "init-file", defaultInitFile(), "path to write init.sh")
	apply.Flags().StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")

	check := &cobra.Command{
		Use:   "check",
		Short: "Validate DAG ordering without writing init.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDAGCheck(cmd, cfg)
		},
	}

	cmd.AddCommand(apply, check)
	return cmd
}

func runDAGApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}

	content := initgen.Generate(ordered, cfg.binDir)
	if cfg.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "# would write %s (%d scripts)\n", cfg.initFile, len(ordered))
		fmt.Fprint(cmd.OutOrStdout(), string(content))
		return nil
	}
	if err := initgen.WriteFile(cfg.initFile, content); err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s wrote %s (%d scripts)\n", ui.Header("init.sh:"), cfg.initFile, len(ordered))
	}
	return nil
}

func runDAGCheck(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d active scripts, %s\n", ui.Header("dag:"), len(ordered), ui.OK("OK"))

	if cfg.verbose {
		for i, n := range ordered {
			fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %s\n", i+1, n.Path)
		}
	}
	return nil
}
