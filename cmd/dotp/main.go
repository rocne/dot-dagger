// Command dotp manages package installation for the dotr suite.
// Standalone mode: unconditional — packages from all files are acted on
// regardless of @when predicates. Use dotr for predicate-filtered installs.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type config struct {
	files string
	dryRun   bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     ecosystem.ToolP,
		Short:   "Dotfiles package manager — installs packages declared via @require/@request (unconditional)",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", defaultDotfiles(), "path to dotfiles repo")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print install commands without executing")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	ui.SetupCobraColors(root)

	root.AddCommand(
		&cobra.Command{
			Use:   "install",
			Short: "Install all packages declared across all files",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runInstall(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Report package status without installing",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all packages declared across all files",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runList(cmd, cfg)
			},
		},
	)
	return root
}

func runInstall(cmd *cobra.Command, cfg *config) error {
	nodes, reg, err := loadContext(cfg)
	if err != nil {
		return err
	}

	for _, req := range packages.CollectRequests(nodes.Nodes) {
		if err := packages.InstallOne(cmd.OutOrStdout(), cmd.ErrOrStderr(), req, reg, cfg.dryRun, cfg.verbose, ecosystem.ToolP, exec.LookPath); err != nil {
			return err
		}
	}
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	nodes, reg, err := loadContext(cfg)
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

func runList(cmd *cobra.Command, cfg *config) error {
	nodes, _, err := loadContext(cfg)
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

// --- helpers ---

func loadContext(cfg *config) (*fileset.Set, *packages.Registry, error) {
	walked, err := walk.Walk(cfg.files)
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	nodes := fileset.BuildUnfiltered(walked)

	reg, err := packages.LoadFile(filepath.Join(cfg.files, "packages.yaml"))
	if err != nil {
		return nil, nil, err
	}

	return nodes, reg, nil
}

func defaultDotfiles() string { return ecosystem.DefaultDotfiles() }
