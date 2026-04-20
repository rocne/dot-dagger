package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/manifest"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package management — filtered by active predicates",
	}
	cmd.AddCommand(
		newPackageGenerateCmd(cfg),
		&cobra.Command{
			Use:   "check",
			Short: "Report package status without installing",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPackageCheck(cfg)
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all packages declared in active nodes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPackageList(cmd, cfg)
			},
		},
	)
	return cmd
}

func newPackageGenerateCmd(cfg *config) *cobra.Command {
	var outputFile string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a shell install script for active package requirements",
		Long: `Generate a shell install script for active package requirements.

Only packages not already installed are included. Package managers and
commands are resolved from packages.yaml — no built-in defaults are used.

Preview packages to install:
  dotd package generate

Install packages:
  dotd package generate | sudo sh

Write install script to file:
  dotd package generate -o packages-install.sh
  sudo sh packages-install.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPackageGenerate(cmd, cfg, outputFile)
		},
	}
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write script to file instead of stdout")
	return cmd
}

// collectAllRequests merges annotation-based and manifest-based package requests.
func collectAllRequests(nodes *fileset.Set) ([]packages.PackageRequest, error) {
	reqs := packages.CollectRequests(nodes.Nodes)
	var paths []string
	for _, n := range nodes.Manifests() {
		paths = append(paths, n.Path)
	}
	mreqs, err := manifest.CollectFromPaths(paths, nodes.Env)
	if err != nil {
		return nil, err
	}
	return append(reqs, mreqs...), nil
}

func runPackageGenerate(cmd *cobra.Command, cfg *config, outputFile string) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	reg, err := loadPackageContext(cfg)
	if err != nil {
		return err
	}
	reqs, err := collectAllRequests(nodes)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("package generate: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
	}

	return packages.GenerateScript(w, reqs, reg, exec.LookPath)
}

func runPackageCheck(cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	reg, err := loadPackageContext(cfg)
	if err != nil {
		return err
	}

	reqs, err := collectAllRequests(nodes)
	if err != nil {
		return err
	}
	if len(reqs) == 0 {
		cfg.log.Info("no package requirements found")
		return nil
	}

	for _, req := range reqs {
		kind := "@request"
		if req.Hard {
			kind = "@require"
		}
		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, exec.LookPath)

		switch {
		case installed:
			cfg.log.Debug(req.Package, "kind", kind, "state", ui.Installed("installed"))
		case installable:
			cfg.log.Info(req.Package, "kind", kind, "state", ui.Installable("installable"))
		case req.Hard:
			cfg.log.Error(req.Package, "kind", kind, "state", ui.HardMissing("MISSING"))
		default:
			cfg.log.Warn(req.Package, "kind", kind, "state", ui.Missing("not available"))
		}
	}
	return nil
}

func runPackageList(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	reqs, err := collectAllRequests(nodes)
	if err != nil {
		return err
	}
	if len(reqs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no package requirements found")
		return nil
	}

	for _, req := range reqs {
		kind := "@request"
		if req.Hard {
			kind = "@require"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s  (%s)\n", kind, req.Package, req.NodePath)
	}
	return nil
}
