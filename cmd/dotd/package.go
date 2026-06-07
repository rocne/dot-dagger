package main

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package management — filtered by active predicates",
	}
	cmd.AddCommand(
		newPackageCheckCmd(cfg),
		newPackageGenerateCmd(cfg),
		newPackageListCmd(cfg),
	)
	return cmd
}

func newPackageCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Report install status for all referenced packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}

			reg, regErr := loadRegistry(cfg)
			if regErr != nil {
				return regErr
			}

			reqs := collectPackageRequests(ordered)
			seen := map[string]bool{}
			for _, r := range reqs {
				if seen[r.Package] {
					continue
				}
				seen[r.Package] = true
				installed, _ := packages.Installed(r.Package, reg, exec.LookPath)
				status := "not installed"
				if installed {
					status = "installed"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s\n", r.Package, status)
			}
			return nil
		},
	}
}

func newPackageGenerateCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate install script for required packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}

			reg, regErr := loadRegistry(cfg)
			if regErr != nil {
				return regErr
			}

			reqs := collectPackageRequests(ordered)
			return packages.GenerateScript(cmd.OutOrStdout(), reqs, reg, exec.LookPath)
		},
	}
}

func newPackageListCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all packages referenced in active nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}

			reqs := collectPackageRequests(ordered)
			seen := map[string]bool{}
			for _, r := range reqs {
				if seen[r.Package] {
					continue
				}
				seen[r.Package] = true
				kind := "request"
				if r.Hard {
					kind = "require"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s\n", r.Package, kind)
			}
			return nil
		},
	}
}

func loadRegistry(cfg *config) (*packages.Registry, error) {
	pkgsFile := filepath.Join(cfg.files, ecosystem.PackagesFileName)
	reg, err := packages.LoadFile(pkgsFile)
	if err != nil {
		return nil, fmt.Errorf("packages: load %s: %w", pkgsFile, err)
	}
	return reg, nil
}

func collectPackageRequests(nodes []pipeline.RawNode) []packages.PackageRequest {
	var reqs []packages.PackageRequest
	for _, n := range nodes {
		if n.IsCompose {
			continue
		}
		for _, pkg := range n.Require {
			reqs = append(reqs, packages.PackageRequest{Package: pkg, Hard: true, NodePath: n.Path})
		}
		for _, pkg := range n.Request {
			reqs = append(reqs, packages.PackageRequest{Package: pkg, Hard: false, NodePath: n.Path})
		}
	}
	return reqs
}

