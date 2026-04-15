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
	envFile  string
	env      []string
	initFile string
	linkRoot string
	binDir   string
	dryRun   bool
	force    bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     "dotr",
		Short:   "Dotfiles orchestrator — composes dote, dotd, dotl, and dotp",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&cfg.envFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&cfg.initFile, "init-file", defaultInitFile(), "path to write init.sh")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	ui.SetupCobraColors(root)

	root.AddCommand(
		newSetupCmd(cfg),
		&cobra.Command{
			Use:   "apply",
			Short: "Full orchestrated reconciliation: env → fileset → packages → links → init.sh",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runApply(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Validate all stages without making changes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd, cfg)
			},
		},
	)
	return root
}

func runApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d keys resolved\n", ui.Header("env:"), len(resolved))
	}

	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d active nodes\n", ui.Header("fileset:"), len(nodes.Nodes))
	}

	reg, priority, err := loadPackageContext(cfg)
	if err != nil {
		return err
	}

	reqs := packages.CollectRequests(nodes.Nodes)
	for _, req := range reqs {
		if err := handlePackage(cmd, cfg, req, reg, priority); err != nil {
			return err
		}
	}

	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.files)

	if cfg.dryRun {
		for _, l := range links {
			if l.State != linker.StateOK {
				fmt.Fprintf(cmd.OutOrStdout(), "# symlink %s %s %s\n", l.Src, ui.Arrow("→"), l.Dst)
			}
		}
	} else {
		if err := linker.Apply(links, cfg.force); err != nil {
			return err
		}
		if cfg.verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d applied\n", ui.Header("symlinks:"), len(links))
		}
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
	} else {
		if err := initgen.WriteFile(cfg.initFile, content); err != nil {
			return err
		}
		if cfg.verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "%s wrote %s (%d scripts)\n", ui.Header("init.sh:"), cfg.initFile, len(ordered))
		}
	}
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}

	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d active nodes\n", ui.Header("fileset:"), len(nodes.Nodes))

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d active, %s\n", ui.Header("scripts:"), len(ordered), ui.OK("DAG OK"))

	reg, priority, err := loadPackageContext(cfg)
	if err != nil {
		return err
	}
	reqs := packages.CollectRequests(nodes.Nodes)
	var pkgMissing int
	for _, req := range reqs {
		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, priority, exec.LookPath)
		if !installed && !installable && req.Hard {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s @require: %s (from %s)\n",
				ui.HardMissing("MISSING"), req.Package, req.NodePath)
			pkgMissing++
		} else if cfg.verbose {
			var status string
			if installed {
				status = ui.Installed("installed")
			} else if installable {
				status = ui.Installable("installable")
			} else {
				status = ui.Missing("not available")
			}
			kind := "@request"
			if req.Hard {
				kind = "@require"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-20s %s\n", kind, req.Package, status)
		}
	}
	if pkgMissing > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", ui.Header("packages:"),
			ui.Conflict(fmt.Sprintf("%d hard requirements unmet", pkgMissing)))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d requirements, %s\n", ui.Header("packages:"), len(reqs), ui.OK("all OK"))
	}

	opts := linker.Options{
		RepoRoot: cfg.files,
		LinkRoot: cfg.linkRoot,
		BinDir:   cfg.binDir,
	}
	links, err := linker.Plan(nodes.Nodes, opts)
	if err != nil {
		return fmt.Errorf("linker plan: %w", err)
	}
	links = linker.Check(links, cfg.files)

	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case linker.StateOK:
			ok++
		case linker.StateMissing:
			missing++
			if cfg.verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Missing("missing"), l.Dst)
			}
		case linker.StateWrongTarget:
			wrong++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Wrong("wrong"), l.Dst)
		case linker.StateConflict:
			conflict++
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Conflict("conflict"), l.Dst)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d ok, %d missing, %d wrong-target, %d conflict\n",
		ui.Header("symlinks:"), ok, missing, wrong, conflict)
	return nil
}

// --- helpers ---

func resolveEnv(cfg *config) (map[string]string, error) {
	ef, err := env.LoadEnvFileFromPath(cfg.envFile)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]string)
	for k, v := range ef.Env {
		overrides[k] = v
	}
	for _, kv := range cfg.env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --env value %q: expected key=value", kv)
		}
		overrides[parts[0]] = parts[1]
	}
	r := env.NewResolver()
	return r.Resolve(overrides)
}

func buildFileSet(cfg *config, resolved map[string]string) (*fileset.Set, error) {
	walked, err := walk.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	return fileset.Build(walked, resolved, nil)
}

func loadPackageContext(cfg *config) (*packages.Registry, []string, error) {
	reg, err := packages.LoadFile(filepath.Join(cfg.files, "packages.yaml"))
	if err != nil {
		return nil, nil, err
	}
	dotrcfg, err := dotryaml.LoadFile(filepath.Join(cfg.files, ".dotr.yaml"))
	if err != nil {
		return nil, nil, err
	}
	return reg, dotrcfg.Dote.PackageManagers.Priority, nil
}

func handlePackage(cmd *cobra.Command, cfg *config, req packages.PackageRequest, reg *packages.Registry, priority []string) error {
	installed, err := packages.Installed(req.Package, reg, exec.LookPath)
	if err != nil {
		return err
	}
	if installed {
		if cfg.verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-14s %s\n", ui.Installed("installed"), req.Package)
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
		if cfg.verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-14s %s (not installable)\n", ui.Skip("skip"), req.Package)
		}
		return nil
	}

	mgr, installCmd, err := resolveInstallCmd(req.Package, reg, priority)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "  %-14s %s via %s: %s\n", ui.Install("install"), req.Package, mgr, installCmd)
	if cfg.dryRun {
		return nil
	}

	c := exec.Command("sh", "-c", installCmd)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		if req.Hard {
			return fmt.Errorf("dotr: install %s: %w", req.Package, err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: install %s: %v\n", req.Package, err)
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
	return filepath.Join(home, ".local", "share", "dot-dagger", "init.sh")
}
