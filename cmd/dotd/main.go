// Command dotd manages dotfiles — env resolution, DAG, symlinks, and packages.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/composer"
	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/initgen"
	"github.com/rocne/dot-dagger/internal/linker"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/predicate"
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
	files        string
	envFile      string
	env          []string
	initFile     string
	linkRoot     string
	binDir       string
	generatedDir string
	dryRun       bool
	force        bool
	verbose      bool
}

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     ecosystem.ToolD,
		Short:   "Dotfiles manager — env resolution, DAG, symlinks, and packages",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", defaultDotfiles(), "path to dotfiles repo")
	pf.StringVar(&cfg.envFile, "env-file", defaultEnvFile(), "path to env.yaml")
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&cfg.initFile, "init-file", defaultInitFile(), "path to write init.sh")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "symlink root for conf/ files (default: $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory for bin/ files")
	pf.StringVar(&cfg.generatedDir, "generated-dir", defaultGeneratedDir(), "path to write composed files")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.BoolVar(&cfg.verbose, "verbose", false, "detailed output")

	ui.SetupCobraColors(root)

	root.AddCommand(
		newSetupCmd(cfg),
		newAdoptCmd(cfg),
		&cobra.Command{
			Use:   "apply",
			Short: "Full reconciliation: env → fileset → packages → links → init.sh",
			Long: `Reconcile the dotfiles repo against the current machine in one shot.

Stages run in order:
  1. env      — detect os/distro/shell, merge env.yaml and --env overrides
  2. fileset  — walk the repo, evaluate @when predicates, build the active set
  3. packages — install packages declared with @require / @request
  4. compose  — assemble compose targets into generated files
  5. links    — create symlinks for conf/ and bin/ files
  6. init.sh  — resolve @after DAG ordering, write init.sh

Examples:
  dotd apply
  dotd apply --dry-run          # preview without making any changes
  dotd apply --env context=work # override a single env key for this run
  dotd apply --verbose          # show per-stage counts`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runApply(cmd, cfg)
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: "Validate all stages without making changes",
			Long: `Validate the dotfiles repo against the current machine without changing anything.

Runs the same evaluation as apply — env resolution, predicate filtering, DAG
ordering, package status, symlink state — and prints a summary of each stage.
Exits non-zero if any stage has errors.

Examples:
  dotd check
  dotd check --verbose   # include per-file symlink status
  dotd check --env os=macos  # preview for a different environment`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCheck(cmd, cfg)
			},
		},
		newEnvCmd(cfg),
		newFilesCmd(cfg),
		newDAGCmd(cfg),
		newLinkCmd(cfg),
		newComposeCmd(cfg),
		newPackageCmd(cfg),
		newCompletionCmd(),
	)
	return root
}

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate shell completion script",
		Long: `Output a shell completion script to source in your shell profile.

  bash:  source <(dotd completion bash)
  zsh:   source <(dotd completion zsh)
  fish:  dotd completion fish | source`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q — choose bash, zsh, fish, or powershell", args[0])
			}
		},
	}
	return cmd
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

	composeOpts := composer.Options{
		GeneratedDir: cfg.generatedDir,
		DryRun:       cfg.dryRun,
	}
	synthetic, err := composer.Apply(nodes.Compose(), composeOpts)
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}
	nodes.Nodes = append(nodes.Nodes, synthetic...)
	if cfg.verbose && len(synthetic) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d generated\n", ui.Header("compose:"), len(synthetic))
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

	composeStatuses, err := composer.Check(nodes.Compose(), composer.Options{GeneratedDir: cfg.generatedDir})
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}
	{
		var ok, missing, stale int
		for _, s := range composeStatuses {
			switch s.State {
			case composer.StateOK:
				ok++
			case composer.StateMissing:
				missing++
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Missing("missing"), s.OutputPath)
			case composer.StateStale:
				stale++
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", ui.Wrong("stale"), s.OutputPath)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d ok, %d missing, %d stale\n",
			ui.Header("compose:"), ok, missing, stale)
		// Add synthetic nodes so linker and initgen see the generated files.
		synthetic, err := composer.Apply(nodes.Compose(), composer.Options{
			GeneratedDir: cfg.generatedDir,
			DryRun:       true, // check mode — do not write
		})
		if err == nil {
			nodes.Nodes = append(nodes.Nodes, synthetic...)
		}
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d active, %s\n", ui.Header("shellrc:"), len(ordered), ui.OK("DAG OK"))

	reg, err := loadPackageContext(cfg)
	if err != nil {
		return err
	}
	reqs, err := collectAllRequests(nodes)
	if err != nil {
		return err
	}
	var pkgMissing int
	for _, req := range reqs {
		installed, _ := packages.Installed(req.Package, reg, exec.LookPath)
		installable, _ := packages.Installable(req.Package, reg, exec.LookPath)
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
	return env.ResolveWithOverrides(cfg.envFile, cfg.env)
}

func buildFileSet(cfg *config, resolved map[string]string) (*fileset.Set, error) {
	walked, err := walk.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	return buildFileSetFromWalked(walked, resolved)
}

func buildFileSetFromWalked(walked []walk.Node, resolved map[string]string) (*fileset.Set, error) {
	set, err := fileset.Build(walked, resolved, nil)
	if err != nil {
		return nil, annotateKeyError(err)
	}
	return set, nil
}

func annotateKeyError(err error) error {
	var mke *predicate.MissingKeyError
	if errors.As(err, &mke) {
		return fmt.Errorf("%w\n\nHint: set it with --env %s=<value> or: dotd env set %s=<value>", err, mke.Key, mke.Key)
	}
	return err
}

func loadPackageContext(cfg *config) (*packages.Registry, error) {
	return packages.LoadFile(filepath.Join(cfg.files, "packages.yaml"))
}

func defaultDotfiles() string { return ecosystem.DefaultDotfiles() }

func defaultGeneratedDir() string {
	p, err := ecosystem.DefaultGeneratedDir()
	if err != nil {
		panic(err)
	}
	return p
}

func defaultEnvFile() string {
	p, err := ecosystem.DefaultEnvFile()
	if err != nil {
		panic(err)
	}
	return p
}

func defaultInitFile() string {
	p, err := ecosystem.DefaultInitFile()
	if err != nil {
		panic(err)
	}
	return p
}
