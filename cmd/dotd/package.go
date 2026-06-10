package main

import (
	"encoding/json"
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
		Long: `Inspect and install packages referenced by @require / @request annotations
on active nodes. Only nodes whose @when predicate evaluates true are considered.

Package definitions live in packages.yaml at the dotfiles repo root.`,
	}
	cmd.AddCommand(
		newPackageCheckCmd(cfg),
		newPackageGenerateCmd(cfg),
		newPackageListCmd(cfg),
	)
	return cmd
}

type packageCheckEntry struct {
	Package   string `json:"package"`
	Installed bool   `json:"installed"`
}

func newPackageCheckCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Report install status for all referenced packages",
		Long: `For each unique package referenced by @require / @request on active nodes,
look it up on PATH via the registry and report installed / not installed.

Examples:
  dotd package check
  dotd package check --env os=macos
  dotd package check --json | jq '.[] | select(.installed == false)'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}

			reg, regErr := loadRegistry(cfg)
			if regErr != nil {
				return regErr
			}

			pkgs := uniquePackages(collectPackageRequests(ordered))
			if jsonOutput {
				entries := make([]packageCheckEntry, len(pkgs))
				for i, r := range pkgs {
					installed, _ := packages.Installed(r.Package, reg, exec.LookPath)
					entries[i] = packageCheckEntry{Package: r.Package, Installed: installed}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, r := range pkgs {
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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}

func newPackageGenerateCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate install script for required packages",
		Long: `Emit a shell script that installs every missing @require / @request
package using the platform's package manager (resolved from packages.yaml).

The script is written to stdout. Pipe to sh to execute, or redirect to a file.

Examples:
  dotd package generate
  dotd package generate > install-packages.sh
  dotd package generate | sh`,
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

type packageListEntry struct {
	Package string `json:"package"`
	Kind    string `json:"kind"` // require | request
}

func newPackageListCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all packages referenced in active nodes",
		Long: `List every unique package referenced by @require or @request on active nodes.

  require   blocks activation if not installed (hard dependency)
  request   installed if missing, but absence doesn't block (soft)

Examples:
  dotd package list
  dotd package list --env os=linux
  dotd package list --json | jq '.[] | select(.kind=="require") | .package'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ordered, err := cfg.walkOrdered(cmd)
			if err != nil {
				return err
			}

			pkgs := uniquePackages(collectPackageRequests(ordered))
			if jsonOutput {
				entries := make([]packageListEntry, len(pkgs))
				for i, r := range pkgs {
					kind := "request"
					if r.Hard {
						kind = "require"
					}
					entries[i] = packageListEntry{Package: r.Package, Kind: kind}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, r := range pkgs {
				kind := "request"
				if r.Hard {
					kind = "require"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s\n", r.Package, kind)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON array")
	return cmd
}

func loadRegistry(cfg *config) (*packages.Registry, error) {
	pkgsFile := filepath.Join(cfg.files, ecosystem.PackagesFileName)
	reg, err := packages.LoadFile(pkgsFile)
	if err != nil {
		return nil, fmt.Errorf("packages: load %s: %w", pkgsFile, err)
	}
	return reg, nil
}

// uniquePackages returns reqs with duplicate Package names removed, first occurrence wins.
func uniquePackages(reqs []packages.PackageRequest) []packages.PackageRequest {
	if reqs == nil {
		return nil
	}
	seen := map[string]bool{}
	out := reqs[:0:0]
	for _, r := range reqs {
		if seen[r.Package] {
			continue
		}
		seen[r.Package] = true
		out = append(out, r)
	}
	return out
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

