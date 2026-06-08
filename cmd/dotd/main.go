// Command dotd manages dotfiles — env resolution, DAG, symlinks, and init.sh generation.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	chlog "github.com/charmbracelet/log"
	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	dotlog "github.com/rocne/dot-dagger/internal/log"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/predicate"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if errors.Is(err, errUserAborted) {
			fmt.Fprintln(os.Stderr, "cancelled")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(1)
	}
}

type appConfig struct {
	files        string
	configPath   string // resolved path to config.yaml — set by resolvePaths, read by config subcommands
	envFile      string
	env          []string
	initFile     string
	linkRoot     string
	binDir       string
	generatedDir string
	dryRun       bool
	force        bool
	logLevel     string
	quiet        bool
	debug        bool
	log          *chlog.Logger
}

// config is a type alias so sub-command constructors can use the short name.
type config = appConfig

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:           ecosystem.ToolD,
		Short:         "Dotfiles manager — env resolution, DAG, symlinks, and init.sh generation",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", "", "path to dotfiles repo (default: $DOTD_FILES → $DOTFILES → config.yaml dotfiles → cwd)")
	pf.StringVar(&cfg.configPath, "config", "", "path to config.yaml (default: $DOTD_CONFIG_FILE → ~/.config/dot-dagger/config.yaml)")
	pf.StringVar(&cfg.envFile, "env-file", "", fmt.Sprintf("path to %s (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/%s)", ecosystem.EnvFileName, ecosystem.EnvFileName))
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&cfg.initFile, "init-file", "", "path to write init.sh (default: $DOTD_INIT_FILE → ~/.local/share/dot-dagger/init.sh)")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "home dir for ~/expansion in link destinations (default: config.yaml link_root → $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory override (default: config.yaml bin_dir → ~/.local/bin/dot-dagger)")
	pf.StringVar(&cfg.generatedDir, "generated-dir", "", "generated files directory (default: config.yaml generated_dir → ~/.local/share/dot-dagger/generated)")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.StringVar(&cfg.logLevel, "log-level", "info", "log verbosity ("+dotlog.LevelNames()+")")
	pf.BoolVar(&cfg.quiet, "quiet", false, "suppress all output except errors")
	pf.BoolVar(&cfg.debug, "debug", false, "set log level to debug (overridden by --log-level)")

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := resolvePaths(cfg); err != nil {
			return err
		}
		return configureLogger(cfg, cmd)
	}

	// dotd help --all reveals hidden internal commands.
	root.PersistentFlags().Bool("all", false, "show all commands including internal helpers")
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		showAll, _ := cmd.Root().PersistentFlags().GetBool("all")
		if showAll {
			for _, sub := range cmd.Root().Commands() {
				sub.Hidden = false
			}
		}
		_ = cmd.Usage()
	})

	ui.SetupCobraColors(root)

	root.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "advanced", Title: "Advanced:"},
	)

	// hidden internal helpers — no group
	root.AddCommand(getOSCmd, getHostnameCmd)

	for _, cmd := range []*cobra.Command{
		newAdoptCmd(cfg),
		newAnnotateCmd(cfg),
		newApplyCmd(cfg),
		newUnapplyCmd(cfg),
		newCheckCmd(cfg),
		newListCmd(cfg),
	} {
		cmd.GroupID = "core"
		root.AddCommand(cmd)
	}

	for _, cmd := range []*cobra.Command{
		newConfigCmd(cfg),
		newEnvCmd(cfg),
		newSetupCmd(cfg),
		newInitCmd(cfg),
		newTeardownCmd(cfg),
	} {
		cmd.GroupID = "config"
		root.AddCommand(cmd)
	}

	for _, cmd := range []*cobra.Command{
		newBundleCmd(cfg),
		newComposeCmd(cfg),
		newDagCmd(cfg),
		newPackageCmd(cfg),
	} {
		cmd.GroupID = "advanced"
		root.AddCommand(cmd)
	}

	root.AddCommand(newCompletionCmd())

	return root
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion <shell>",
		Short:     "Generate shell completion script",
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
}

// resolveEnv builds the runtime env map for predicate evaluation.
// Precedence: --env flags > DOTD_* shell vars > env.yaml (expanded).
func resolveEnv(cfg *config) (map[string]string, error) {
	raw, err := env.Load(cfg.envFile)
	if err != nil {
		return nil, fmt.Errorf("env: load %s: %w", cfg.envFile, err)
	}
	expanded, err := env.Expand(raw)
	if err != nil {
		return nil, fmt.Errorf("env: expand: %w", err)
	}
	shellVars := env.ShellVars(os.Environ())

	cliFlags := map[string]string{}
	for _, e := range cfg.env {
		idx := strings.IndexByte(e, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("--env: invalid format %q (want key=value)", e)
		}
		cliFlags[e[:idx]] = e[idx+1:]
	}

	return env.Resolve(cliFlags, shellVars, expanded), nil
}

// resolvePaths fills in all empty path fields in cfg.
// Precedence: CLI flag > DOTD_* env var > config.yaml field > XDG/system default.
func resolvePaths(cfg *config) error {
	var err error

	// env-file first — no config.yaml lookup (would be circular).
	cfg.envFile, err = ecosystem.ResolvePath(cfg.envFile, "DOTD_ENV_FILE", "", env.DefaultPath)
	if err != nil {
		return err
	}

	// Tool preferences from config.yaml. Path stored in cfg so config subcommands don't re-resolve it.
	// No config-file value tier (third arg "") — config.yaml is what we're resolving; would be circular.
	cfg.configPath, err = ecosystem.ResolvePath(cfg.configPath, "DOTD_CONFIG_FILE", "", ecosystem.DefaultConfigFile)
	if err != nil {
		return err
	}
	toolCfg, err := dotcfg.Load(cfg.configPath)
	if err != nil {
		return err
	}

	cfg.files, err = ecosystem.ResolvePath(cfg.files, "DOTD_FILES", toolCfg.Dotfiles, func() (string, error) {
		return ecosystem.DefaultDotfiles(), nil
	})
	if err != nil {
		return err
	}

	cfg.initFile, err = ecosystem.ResolvePath(cfg.initFile, "DOTD_INIT_FILE", "", ecosystem.DefaultInitFile)
	if err != nil {
		return err
	}

	cfg.linkRoot, err = ecosystem.ResolvePath(cfg.linkRoot, "DOTD_LINK_ROOT", toolCfg.LinkRoot, ecosystem.DefaultLinkRoot)
	if err != nil {
		return err
	}

	cfg.binDir, err = ecosystem.ResolvePath(cfg.binDir, "DOTD_BIN_DIR", toolCfg.BinDir, ecosystem.DefaultBinDir)
	if err != nil {
		return err
	}

	cfg.generatedDir, err = ecosystem.ResolvePath(cfg.generatedDir, "DOTD_GENERATED_DIR", toolCfg.GeneratedDir, ecosystem.DefaultGeneratedDir)
	if err != nil {
		return err
	}

	return nil
}

// resolveLogLevel determines effective log level from three sources.
// Priority (highest wins): quiet > explicit --log-level > --debug > default.
func resolveLogLevel(logLevel string, debug bool, logLevelExplicit bool, quiet bool) string {
	level := logLevel
	if debug && !logLevelExplicit {
		level = "debug"
	}
	if quiet {
		level = "error"
	}
	return level
}

func configureLogger(cfg *config, cmd *cobra.Command) error {
	logLevelExplicit := cmd.Root().PersistentFlags().Changed("log-level")
	level := resolveLogLevel(cfg.logLevel, cfg.debug, logLevelExplicit, cfg.quiet)
	logger, err := dotlog.New(cmd.ErrOrStderr(), level)
	if err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	cfg.log = logger
	return nil
}

// buildActOptions constructs pipeline.ActOptions from cfg.
// dryRun forces dry-run mode regardless of cfg.dryRun.
// cfg.linkRoot is guaranteed non-empty after resolvePaths succeeds.
func buildActOptions(cfg *config, dryRun bool) (pipeline.ActOptions, error) {
	return pipeline.ActOptions{
		HomeDir:      cfg.linkRoot,
		BinDir:       cfg.binDir,
		GeneratedDir: cfg.generatedDir,
		DryRun:       dryRun || cfg.dryRun,
		Force:        cfg.force,
	}, nil
}

type pipelineRun struct {
	resolvedCount int
	allCount      int
	activeCount   int
	result        *pipeline.ActResult
}

func runPipeline(cmd *cobra.Command, cfg *config, dryRun bool) (*pipelineRun, error) {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return nil, annotateKeyError(err)
	}

	nodes, disabled, err := pipeline.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	for _, p := range disabled {
		cfg.log.Debugf("disabled: %s", p)
	}

	if err := pipeline.ValidateNodes(nodes, pipeline.ActOptions{HomeDir: cfg.linkRoot, BinDir: cfg.binDir}); err != nil {
		return nil, err
	}

	active, err := filterWithPrompt(cmd, nodes, resolved, isTTY(cmd.InOrStdin()))
	if err != nil {
		return nil, err
	}

	ordered, err := pipeline.Order(active)
	if err != nil {
		return nil, fmt.Errorf("order: %w", err)
	}

	actOpts, err := buildActOptions(cfg, dryRun)
	if err != nil {
		return nil, err
	}
	res, err := pipeline.Act(ordered, actOpts)
	if err != nil {
		return nil, fmt.Errorf("act: %w", err)
	}

	return &pipelineRun{
		resolvedCount: len(resolved),
		allCount:      len(nodes),
		activeCount:   len(active),
		result:        res,
	}, nil
}

// walkOrdered runs the read-path pipeline preamble:
//
//	resolveEnv → Walk → filterWithPrompt → Order → ValidateNodes
//
// Returns the ordered active node slice ready for read-only commands.
// Write-path commands use runPipeline instead, which additionally performs Act.
//
// Validation is shared with the write path so a config that apply/check
// rejects also fails under list/dag/bundle/compose/package.
func (cfg *appConfig) walkOrdered(cmd *cobra.Command) ([]pipeline.RawNode, error) {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return nil, annotateKeyError(err)
	}
	nodes, _, err := pipeline.Walk(cfg.files)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	active, err := filterWithPrompt(cmd, nodes, resolved, isTTY(cmd.InOrStdin()))
	if err != nil {
		return nil, err
	}
	ordered, err := pipeline.Order(active)
	if err != nil {
		return nil, fmt.Errorf("order: %w", err)
	}
	if err := pipeline.ValidateNodes(ordered, pipeline.ActOptions{HomeDir: cfg.linkRoot, BinDir: cfg.binDir}); err != nil {
		return nil, err
	}
	return ordered, nil
}

func annotateKeyError(err error) error {
	var mke *predicate.MissingKeyError
	if errors.As(err, &mke) {
		return fmt.Errorf("%w\n\nHint: set it with --env %s=<value> or add it to %s", err, mke.Key, ecosystem.EnvFileName)
	}
	return err
}

func newApplyCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Reconcile dotfiles: walk → filter → order → act → init.sh",
		Long: fmt.Sprintf(`Reconcile the dotfiles repo against the current machine.

Stages run in order:
  1. env    — load %s, merge DOTD_* shell vars and --env overrides
  2. walk   — traverse dotfiles repo, load .dagger configs, produce raw nodes
  3. filter — evaluate @when predicates against resolved env
  4. order  — topological sort via @after annotations (alphabetical tie-break)
  5. act    — create symlinks, collect source list
  6. init   — write init.sh from sourced nodes in DAG order

Examples:
  dotd apply
  dotd apply --dry-run            # preview without making changes
  dotd apply --env context=work   # override a single env key`, ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, cfg)
		},
	}
}

func newCheckCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate pipeline stages without writing anything",
		Long: `Run the full pipeline in dry-run mode and report filesystem state.

Exits non-zero if any stage reports issues (missing symlinks, missing init.sh, etc.).

Examples:
  dotd check
  dotd check --env os=macos   # preview for a different environment`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, cfg)
		},
	}
}

// warnIfNosyncUnignored warns about nosync- paths in the dotfiles repo that are not gitignored.
func warnIfNosyncUnignored(cfg *config) {
	_ = filepath.WalkDir(cfg.files, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "nosync-") {
			return nil
		}
		// git check-ignore exits 0 if ignored, 1 if not, 128 if not a git repo.
		cmd := exec.Command("git", "-C", cfg.files, "check-ignore", "--quiet", path)
		if cmd.Run() != nil {
			cfg.log.Warnf("nosync- path not gitignored: %s — add to .gitignore to avoid committing private files", path)
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
}

func runApply(cmd *cobra.Command, cfg *config) error {
	warnIfNosyncUnignored(cfg)
	run, err := runPipeline(cmd, cfg, false)
	if err != nil {
		return err
	}
	cfg.log.Debugf("%s %d keys", ui.Header("env:"), run.resolvedCount)
	cfg.log.Debugf("%s %d active / %d total", ui.Header("filter:"), run.activeCount, run.allCount)

	if cfg.dryRun {
		for _, lnk := range run.result.Links {
			fmt.Fprintf(cmd.OutOrStdout(), "# link %s %s %s\n", lnk.Src, ui.Arrow("→"), lnk.Dest)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# would write %s (%d sourced nodes)\n", cfg.initFile, len(run.result.Sourced))
		return nil
	}

	cfg.log.Infof("%s %d symlinks applied", ui.Header("links:"), len(run.result.Links))

	if err := pipeline.Generate(cfg.initFile, run.result.Sourced, pipeline.GenerateOptions{
		BinDir: cfg.binDir,
	}); err != nil {
		return fmt.Errorf("generate init.sh: %w", err)
	}
	cfg.log.Infof("%s wrote %s (%d nodes)", ui.Header("init.sh:"), cfg.initFile, len(run.result.Sourced))
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	warnIfNosyncUnignored(cfg)
	run, err := runPipeline(cmd, cfg, true)
	if err != nil {
		return err
	}
	cfg.log.Infof("%s %d active / %d total", ui.Header("filter:"), run.activeCount, run.allCount)

	var hasIssues bool

	var ok, missing, wrong int
	for _, lnk := range run.result.Links {
		target, lerr := os.Readlink(lnk.Dest)
		if errors.Is(lerr, fs.ErrNotExist) {
			missing++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "missing")
		} else if lerr != nil {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "conflict")
		} else if target != lnk.Src {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "wrong-target", "want", lnk.Src, "got", target)
		} else {
			ok++
		}
	}
	cfg.log.Infof("%s %d %s, %d %s, %d %s", ui.Header("symlinks:"), ok, ui.OK("ok"), missing, ui.Missing("missing"), wrong, ui.Wrong("wrong"))

	if _, serr := os.Stat(cfg.initFile); errors.Is(serr, fs.ErrNotExist) {
		cfg.log.Warn(cfg.initFile, "state", "missing")
		hasIssues = true
	} else {
		cfg.log.Infof("%s %s present (%d sourced nodes)", ui.Header("init.sh:"), cfg.initFile, len(run.result.Sourced))
	}

	if hasIssues {
		cmd.SilenceErrors = true
		return errors.New("check: issues found")
	}
	return nil
}

// launchEditor opens path in $EDITOR, falling back to vi.
func launchEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

