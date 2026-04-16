package main

import (
	"fmt"
	"os/exec"

	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newPackageCmd(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package management — predicate-filtered (see dotp for unconditional)",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "install",
			Short: "Install all packages declared in active nodes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPackageInstall(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Report package status without installing",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPackageCheck(cmd, cfg)
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

func runPackageInstall(cmd *cobra.Command, cfg *config) error {
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
	reqs := packages.CollectRequests(nodes.Nodes)
	for _, req := range reqs {
		if err := handlePackage(cmd, cfg, req, reg); err != nil {
			return err
		}
	}
	return nil
}

func runPackageCheck(cmd *cobra.Command, cfg *config) error {
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

	reqs := packages.CollectRequests(nodes.Nodes)
	if len(reqs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no package requirements found")
		return nil
	}

	for _, req := range reqs {
		kind := "@request"
		if req.Hard {
			kind = "@require"
		}
		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, exec.LookPath)

		var status string
		switch {
		case installed:
			status = ui.Installed("installed")
		case installable:
			status = ui.Installable("installable")
		case req.Hard:
			status = ui.HardMissing("MISSING") + " (hard requirement)"
		default:
			status = ui.Missing("not available")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-20s %s\n", kind, req.Package, status)
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

	reqs := packages.CollectRequests(nodes.Nodes)
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
