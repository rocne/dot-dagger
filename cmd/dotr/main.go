// Command dotr is the orchestrator for the dotr suite.
// It composes dote, dotd, dotl, and dotp into a single reconciliation pass.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/dotryaml"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/initgen"
	"github.com/rocne/dot-dagger/internal/linker"
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
	Use:   "dotr",
	Short: "Dotfiles orchestrator — composes dote, dotd, dotl, and dotp",
}

var (
	flagDotfiles string
	flagEnvFile  string
	flagEnv      []string
	flagInitFile string
	flagLinkRoot string
	flagBinDir   string
	flagDryRun   bool
	flagForce    bool
	flagVerbose  bool
)

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagDotfiles, "dotfiles", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&flagEnvFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&flagEnv, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&flagInitFile, "init-file", defaultInitFile(), "path to write init.sh")
	pf.StringVar(&flagLinkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&flagBinDir, "bin-dir", "", "bin directory for bin/ files")
	pf.BoolVar(&flagDryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&flagForce, "force", false, "override safety checks")
	pf.BoolVar(&flagVerbose, "verbose", false, "detailed output")

	rootCmd.AddCommand(applyCmd, checkCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Full orchestrated reconciliation: env → fileset → packages → links → init.sh",
	RunE:  runApply,
}

func runApply(cmd *cobra.Command, args []string) error {
	// 1. Resolve environment.
	resolved, err := resolveEnv()
	if err != nil {
		return err
	}
	if flagVerbose {
		fmt.Printf("env: %d keys resolved\n", len(resolved))
	}

	// 2. Walk + build fileset.
	nodes, err := buildFileSet(resolved)
	if err != nil {
		return err
	}
	if flagVerbose {
		fmt.Printf("fileset: %d active nodes\n", len(nodes.Nodes))
	}

	// 3. Load package registry and priority.
	reg, priority, err := loadPackageContext()
	if err != nil {
		return err
	}

	// 4. Install packages (dotp step).
	reqs := packages.CollectRequests(nodes.Nodes)
	for _, req := range reqs {
		if err := handlePackage(req, reg, priority); err != nil {
			return err
		}
	}

	// 5. Apply symlinks (dotl step).
	opts := linker.Options{
		RepoRoot: flagDotfiles,
		LinkRoot: flagLinkRoot,
		BinDir:   flagBinDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, flagDotfiles)

	if flagDryRun {
		for _, l := range links {
			if l.State != linker.StateOK {
				fmt.Printf("# symlink %s → %s\n", l.Src, l.Dst)
			}
		}
	} else {
		if err := linker.Apply(links, flagForce); err != nil {
			return err
		}
		if flagVerbose {
			fmt.Printf("symlinks: %d applied\n", len(links))
		}
	}

	// 6. Generate init.sh (dotd step).
	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	content := initgen.Generate(ordered, flagBinDir)

	if flagDryRun {
		fmt.Printf("# would write %s (%d scripts)\n", flagInitFile, len(ordered))
		fmt.Print(string(content))
	} else {
		if err := initgen.WriteFile(flagInitFile, content); err != nil {
			return err
		}
		if flagVerbose {
			fmt.Printf("init.sh: wrote %s (%d scripts)\n", flagInitFile, len(ordered))
		}
	}
	return nil
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate all stages without making changes",
	RunE:  runCheck,
}

func runCheck(cmd *cobra.Command, args []string) error {
	resolved, err := resolveEnv()
	if err != nil {
		return err
	}

	nodes, err := buildFileSet(resolved)
	if err != nil {
		return err
	}
	fmt.Printf("fileset: %d active nodes\n", len(nodes.Nodes))

	// DAG check.
	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Printf("scripts: %d active, DAG OK\n", len(ordered))

	// Package check.
	reg, priority, err := loadPackageContext()
	if err != nil {
		return err
	}
	reqs := packages.CollectRequests(nodes.Nodes)
	var pkgMissing int
	for _, req := range reqs {
		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, priority, exec.LookPath)
		if !installed && !installable && req.Hard {
			fmt.Printf("  MISSING @require: %s (from %s)\n", req.Package, req.NodePath)
			pkgMissing++
		} else if flagVerbose {
			status := "not available"
			if installed {
				status = "installed"
			} else if installable {
				status = "installable"
			}
			kind := "@request"
			if req.Hard {
				kind = "@require"
			}
			fmt.Printf("  %-10s %-20s %s\n", kind, req.Package, status)
		}
	}
	if pkgMissing > 0 {
		fmt.Printf("packages: %d hard requirements unmet\n", pkgMissing)
	} else {
		fmt.Printf("packages: %d requirements, all OK\n", len(reqs))
	}

	// Symlink check.
	opts := linker.Options{
		RepoRoot: flagDotfiles,
		LinkRoot: flagLinkRoot,
		BinDir:   flagBinDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, flagDotfiles)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
		case linker.StateMissing:
			missing++
			if flagVerbose {
				fmt.Printf("  missing  %s\n", l.Dst)
			}
		case linker.StateWrongTarget:
			wrong++
			fmt.Printf("  wrong    %s\n", l.Dst)
		case linker.StateConflict:
			conflict++
			fmt.Printf("  conflict %s\n", l.Dst)
		}
	}
	fmt.Printf("symlinks: %d ok, %d missing, %d wrong-target, %d conflict\n",
		ok, missing, wrong, conflict)
	return nil
}

// --- helpers ---

func resolveEnv() (map[string]string, error) {
	ef, err := env.LoadEnvFileFromPath(flagEnvFile)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range flagEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}
	r := env.NewResolver()
	return r.Resolve(overrides)
}

func buildFileSet(resolved map[string]string) (*fileset.Set, error) {
	walked, err := walk.Walk(flagDotfiles)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", flagDotfiles, err)
	}
	return fileset.Build(walked, resolved, nil)
}

func loadPackageContext() (*packages.Registry, []string, error) {
	reg, err := packages.LoadFile(filepath.Join(flagDotfiles, "packages.yaml"))
	if err != nil {
		return nil, nil, err
	}
	cfg, err := dotryaml.LoadFile(filepath.Join(flagDotfiles, ".dotr.yaml"))
	if err != nil {
		return nil, nil, err
	}
	return reg, cfg.Dote.PackageManagers.Priority, nil
}

func handlePackage(req packages.PackageRequest, reg *packages.Registry, priority []string) error {
	installed, err := packages.Installed(req.Package, reg, exec.LookPath)
	if err != nil {
		return err
	}
	if installed {
		if flagVerbose {
			fmt.Printf("  installed  %s\n", req.Package)
		}
		return nil
	}

	installable, err := packages.Installable(req.Package, reg, priority, exec.LookPath)
	if err != nil {
		return err
	}

	if !installable {
		if req.Hard {
			return fmt.Errorf("dotr: %s: @require %q: not installed and not installable",
				req.NodePath, req.Package)
		}
		if flagVerbose {
			fmt.Printf("  skip       %s (not installable)\n", req.Package)
		}
		return nil
	}

	mgr, installCmd, err := resolveInstallCmd(req.Package, reg, priority)
	if err != nil {
		return err
	}

	fmt.Printf("  install    %s via %s: %s\n", req.Package, mgr, installCmd)
	if flagDryRun {
		return nil
	}

	c := exec.Command("sh", "-c", installCmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		if req.Hard {
			return fmt.Errorf("dotr: install %s: %w", req.Package, err)
		}
		fmt.Fprintf(os.Stderr, "warning: install %s: %v\n", req.Package, err)
	}
	return nil
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
	return "", "", fmt.Errorf("dotr: no installable manager found for %q", pkg)
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

func defaultInitFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dot-dagger", "init.sh")
}
