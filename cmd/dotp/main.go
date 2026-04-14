// Command dotp manages package installation for the dotr suite.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/dotryaml"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "dotp",
	Short: "Dotfiles package manager — installs packages declared via @require/@request",
}

var (
	flagDotfiles string
	flagEnvFile  string
	flagEnv      []string
	flagDryRun   bool
	flagVerbose  bool
)

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagDotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&flagEnvFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&flagEnv, "env", nil, "env override as key=value (repeatable)")
	pf.BoolVar(&flagDryRun, "dry-run", false, "print install commands without executing")
	pf.BoolVar(&flagVerbose, "verbose", false, "detailed output")

	rootCmd.AddCommand(installCmd, checkCmd, listCmd)
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install all packages for active files",
	RunE:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	nodes, reg, priority, err := loadContext()
	if err != nil {
		return err
	}

	reqs := packages.CollectRequests(nodes.Nodes)
	for _, req := range reqs {
		installed, err := packages.Installed(req.Package, reg, exec.LookPath)
		if err != nil {
			return err
		}
		if installed {
			if flagVerbose {
				fmt.Printf("  installed  %s\n", req.Package)
			}
			continue
		}

		installable, err := packages.Installable(req.Package, reg, priority, exec.LookPath)
		if err != nil {
			return err
		}

		if !installable {
			if req.Hard {
				return fmt.Errorf("dotp: %s: @require %q: not installed and not installable",
					req.NodePath, req.Package)
			}
			// @request: silently skip.
			if flagVerbose {
				fmt.Printf("  skip       %s (not installable)\n", req.Package)
			}
			continue
		}

		mgr, installCmd, err := resolveInstallCmd(req.Package, reg, priority)
		if err != nil {
			return err
		}

		fmt.Printf("  install    %s via %s: %s\n", req.Package, mgr, installCmd)
		if flagDryRun {
			continue
		}

		if err := runShellCmd(installCmd); err != nil {
			if req.Hard {
				return fmt.Errorf("dotp: install %s: %w", req.Package, err)
			}
			fmt.Fprintf(os.Stderr, "warning: install %s: %v\n", req.Package, err)
		}
	}
	return nil
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Report package status without installing",
	RunE:  runCheck,
}

func runCheck(cmd *cobra.Command, args []string) error {
	nodes, reg, priority, err := loadContext()
	if err != nil {
		return err
	}

	reqs := packages.CollectRequests(nodes.Nodes)
	if len(reqs) == 0 {
		fmt.Println("no package requirements found")
		return nil
	}

	for _, req := range reqs {
		kind := "@request"
		if req.Hard {
			kind = "@require"
		}

		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, priority, exec.LookPath)

		var status string
		switch {
		case installed:
			status = "installed"
		case installable:
			status = "installable"
		case req.Hard:
			status = "MISSING (hard requirement)"
		default:
			status = "not available"
		}

		fmt.Printf("  %-10s %-20s %s\n", kind, req.Package, status)
	}
	return nil
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all packages declared across active files",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	nodes, _, _, err := loadContext()
	if err != nil {
		return err
	}

	reqs := packages.CollectRequests(nodes.Nodes)
	if len(reqs) == 0 {
		fmt.Println("no package requirements found")
		return nil
	}

	for _, req := range reqs {
		kind := "@request"
		if req.Hard {
			kind = "@require"
		}
		fmt.Printf("%-10s %s  (%s)\n", kind, req.Package, req.NodePath)
	}
	return nil
}

// --- helpers ---

func loadContext() (*fileset.Set, *packages.Registry, []string, error) {
	ef, err := env.LoadEnvFileFromPath(flagEnvFile)
	if err != nil {
		return nil, nil, nil, err
	}
	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range flagEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, nil, nil, fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}
	r := env.NewResolver()
	resolved, err := r.Resolve(overrides)
	if err != nil {
		return nil, nil, nil, err
	}

	walked, err := walk.Walk(flagDotfiles)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("walk %s: %w", flagDotfiles, err)
	}
	nodes, err := fileset.Build(walked, resolved, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	reg, err := packages.LoadFile(filepath.Join(flagDotfiles, "packages.yaml"))
	if err != nil {
		return nil, nil, nil, err
	}

	cfg, err := dotryaml.LoadFile(filepath.Join(flagDotfiles, ".dotr.yaml"))
	if err != nil {
		return nil, nil, nil, err
	}
	priority := cfg.Dote.PackageManagers.Priority

	return nodes, reg, priority, nil
}

func resolveInstallCmd(pkg string, reg *packages.Registry, priority []string) (string, string, error) {
	for _, mgr := range priority {
		entry, ok := reg.Packages[pkg]
		if !ok {
			continue
		}
		if _, hasEntry := entry.Managers[mgr]; !hasEntry {
			continue
		}
		if _, err := exec.LookPath(mgr); err != nil {
			continue
		}
		cmd, err := packages.InstallCmd(pkg, mgr, reg)
		if err != nil {
			return "", "", err
		}
		return mgr, cmd, nil
	}
	return "", "", fmt.Errorf("dotp: no installable manager found for %q", pkg)
}

func runShellCmd(cmdStr string) error {
	c := exec.Command("sh", "-c", cmdStr)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func defaultDotfiles() string {
	if d, ok := os.LookupEnv("DOTFILES"); ok {
		return d
	}
	dir, _ := os.Getwd()
	return dir
}

func defaultEnvFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dot-dagger", "env.yaml")
}
