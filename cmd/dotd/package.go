package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "package",
		Short:  "Package management — filtered by active predicates",
		Hidden: true,
	}
	cmd.AddCommand(
		newPackageCheckCmd(cfg),
		newPackageGenerateCmd(cfg),
	)
	return cmd
}

func newPackageCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Report install status for all referenced packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, err := pipeline.Walk(cfg.files)
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
			active, err := pipeline.Filter(nodes, resolved)
			if err != nil {
				return annotateKeyError(err)
			}

			reg, regErr := loadRegistry(cfg)
			if regErr != nil {
				return regErr
			}

			reqs := collectPackageRequests(active)
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
			resolved, err := resolveEnv(cfg)
			if err != nil {
				return annotateKeyError(err)
			}
			nodes, err := pipeline.Walk(cfg.files)
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
			active, err := pipeline.Filter(nodes, resolved)
			if err != nil {
				return annotateKeyError(err)
			}

			reg, regErr := loadRegistry(cfg)
			if regErr != nil {
				return regErr
			}

			reqs := collectPackageRequests(active)
			return packages.GenerateScript(cmd.OutOrStdout(), reqs, reg, exec.LookPath)
		},
	}
}

func loadRegistry(cfg *config) (*packages.Registry, error) {
	pkgsFile := filepath.Join(cfg.files, "packages.yaml")
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
		anns, err := scanAnnotations(n.Path)
		if err != nil {
			continue
		}
		for _, a := range annotation.Get(anns, "require") {
			if a.Args != "" {
				reqs = append(reqs, packages.PackageRequest{Package: a.Args, Hard: true, NodePath: n.Path})
			}
		}
		for _, a := range annotation.Get(anns, "request") {
			if a.Args != "" {
				reqs = append(reqs, packages.PackageRequest{Package: a.Args, Hard: false, NodePath: n.Path})
			}
		}
	}
	return reqs
}

func scanAnnotations(path string) ([]annotation.Annotation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return annotation.Scan(f)
}
