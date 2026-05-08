// Command dotd manages dotfiles — env resolution, DAG, symlinks, and init.sh generation.
package main

import (
	"errors"
	"fmt"
	"os"
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
		os.Exit(1)
	}
}

type appConfig struct {
	files        string
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
	log          *chlog.Logger
}

// config is a type alias so sub-command constructors can use the short name.
type config = appConfig

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:     ecosystem.ToolD,
		Short:   "Dotfiles manager — env resolution, DAG, symlinks, and init.sh generation",
		Version: version,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", "", "path to dotfiles repo (default: $DOTD_FILES → $DOTFILES → config.yaml dotfiles → cwd)")
	pf.StringVar(&cfg.envFile, "env-file", "", "path to env.yaml (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/env.yaml)")
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.StringVar(&cfg.initFile, "init-file", "", "path to write init.sh (default: $DOTD_INIT_FILE → ~/.local/share/dot-dagger/init.sh)")
	pf.StringVar(&cfg.linkRoot, "link-root", "", "symlink root override (default: config.yaml link_root → $HOME)")
	pf.StringVar(&cfg.binDir, "bin-dir", "", "bin directory override (default: config.yaml bin_dir → ~/.local/bin/dot-dagger)")
	pf.StringVar(&cfg.generatedDir, "generated-dir", "", "generated files directory (default: config.yaml generated_dir → ~/.local/share/dot-dagger/generated)")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.StringVar(&cfg.logLevel, "log-level", "info", "log verbosity ("+dotlog.LevelNames()+")")
	pf.BoolVar(&cfg.quiet, "quiet", false, "suppress all output except errors")

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

	root.AddCommand(
		getOSCmd,
		getHostnameCmd,
		newConfigCmd(),
		newSetupCmd(cfg),
		newAdoptCmd(cfg),
		newApplyCmd(cfg),
		newCheckCmd(cfg),
		newEnvCmd(cfg),
		newBundleCmd(cfg),
		newPackageCmd(cfg),
		newCompletionCmd(),
	)
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

	// Tool preferences from config.yaml.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	toolCfg, err := dotcfg.Load(configPath)
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

func configureLogger(cfg *config, cmd *cobra.Command) error {
	level := cfg.logLevel
	if cfg.quiet {
		level = "error"
	}
	logger, err := dotlog.New(cmd.ErrOrStderr(), level)
	if err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	cfg.log = logger
	return nil
}

func annotateKeyError(err error) error {
	var mke *predicate.MissingKeyError
	if errors.As(err, &mke) {
		return fmt.Errorf("%w\n\nHint: set it with --env %s=<value> or add it to env.yaml", err, mke.Key)
	}
	return err
}

func newApplyCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Reconcile dotfiles: walk → filter → order → act → init.sh",
		Long: `Reconcile the dotfiles repo against the current machine.

Stages run in order:
  1. env    — load env.yaml, merge DOTD_* shell vars and --env overrides
  2. walk   — traverse dotfiles repo, load .dagger configs, produce raw nodes
  3. filter — evaluate @when predicates against resolved env
  4. order  — topological sort via @after annotations (alphabetical tie-break)
  5. act    — create symlinks, collect source list
  6. init   — write init.sh from sourced nodes in DAG order

Examples:
  dotd apply
  dotd apply --dry-run            # preview without making changes
  dotd apply --env context=work   # override a single env key`,
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

func runApply(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return annotateKeyError(err)
	}
	cfg.log.Debugf("%s %d keys", ui.Header("env:"), len(resolved))

	nodes, err := pipeline.Walk(cfg.files)
	if err != nil {
		return fmt.Errorf("walk %s: %w", cfg.files, err)
	}

	active, err := pipeline.Filter(nodes, resolved)
	if err != nil {
		return annotateKeyError(err)
	}
	cfg.log.Debugf("%s %d active / %d total", ui.Header("filter:"), len(active), len(nodes))

	ordered, err := pipeline.Order(active)
	if err != nil {
		return fmt.Errorf("order: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	res, err := pipeline.Act(ordered, pipeline.ActOptions{HomeDir: home, DryRun: cfg.dryRun})
	if err != nil {
		return fmt.Errorf("act: %w", err)
	}

	if cfg.dryRun {
		for _, lnk := range res.Links {
			fmt.Fprintf(cmd.OutOrStdout(), "# link %s %s %s\n", lnk.Src, ui.Arrow("→"), lnk.Dest)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# would write %s (%d sourced nodes)\n", cfg.initFile, len(res.Sourced))
		return nil
	}

	cfg.log.Infof("%s %d symlinks applied", ui.Header("links:"), len(res.Links))

	if err := pipeline.Generate(cfg.initFile, res.Sourced); err != nil {
		return fmt.Errorf("generate init.sh: %w", err)
	}
	cfg.log.Infof("%s wrote %s (%d nodes)", ui.Header("init.sh:"), cfg.initFile, len(res.Sourced))
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return annotateKeyError(err)
	}

	nodes, err := pipeline.Walk(cfg.files)
	if err != nil {
		return fmt.Errorf("walk %s: %w", cfg.files, err)
	}

	active, err := pipeline.Filter(nodes, resolved)
	if err != nil {
		return annotateKeyError(err)
	}
	cfg.log.Infof("%s %d active / %d total", ui.Header("filter:"), len(active), len(nodes))

	ordered, err := pipeline.Order(active)
	if err != nil {
		return fmt.Errorf("order: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	res, err := pipeline.Act(ordered, pipeline.ActOptions{HomeDir: home, DryRun: true})
	if err != nil {
		return fmt.Errorf("act: %w", err)
	}

	var hasIssues bool

	var ok, missing, wrong int
	for _, lnk := range res.Links {
		target, lerr := os.Readlink(lnk.Dest)
		if os.IsNotExist(lerr) {
			missing++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "missing")
		} else if lerr != nil {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "not-a-symlink")
		} else if target != lnk.Src {
			wrong++
			hasIssues = true
			cfg.log.Warn(lnk.Dest, "state", "wrong-target", "want", lnk.Src, "got", target)
		} else {
			ok++
		}
	}
	cfg.log.Infof("%s %d ok, %d missing, %d wrong", ui.Header("symlinks:"), ok, missing, wrong)

	if _, serr := os.Stat(cfg.initFile); os.IsNotExist(serr) {
		cfg.log.Warn(cfg.initFile, "state", "missing")
		hasIssues = true
	} else {
		cfg.log.Infof("%s %s present (%d sourced nodes)", ui.Header("init.sh:"), cfg.initFile, len(res.Sourced))
	}

	if hasIssues {
		cmd.SilenceErrors = true
		return errors.New("check: issues found")
	}
	return nil
}

