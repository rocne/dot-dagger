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
	"github.com/rocne/dot-dagger/internal/fileutil"
	dotlog "github.com/rocne/dot-dagger/internal/log"
	"github.com/rocne/dot-dagger/internal/node"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/rocne/dot-dagger/internal/predicate"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version, commit, and date are the build metadata reported by `dotd --version`.
// They default to dev-build placeholders and are overridden at release time via
// goreleaser ldflags -X main.version / -X main.commit / -X main.date (see
// .goreleaser/dotd.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// pathFlagOwners restricts which subcommands advertise each persistent
// path/mutation flag in --help. The flag stays registered on the root
// PersistentFlags (so cfg paths still resolve uniformly via resolvePaths),
// but Cobra's "Global Flags" block hides it on commands that don't act
// on it — e.g. `dotd config get` no longer surfaces --dry-run.
//
// Keys are the full command path ("dotd apply", "dotd dag order"). The
// root command and `dotd help` are intentionally absent: root --help
// shows every flag so users can discover them.
var pathFlagOwners = map[string]map[string]bool{
	"dry-run": {
		"dotd apply": true, "dotd adopt": true,
		"dotd unapply": true, "dotd teardown": true,
	},
	"force": {
		"dotd apply": true, "dotd adopt": true,
	},
}

// hideIrrelevantInheritedFlags toggles Hidden on inherited persistent flags
// so cmd's --help only advertises flags relevant to it. Returns a restore
// func that resets the prior Hidden state — callers should defer it.
//
// Root and the built-in `help` command get every flag (no filtering), so
// `dotd --help` remains the canonical flag reference.
func hideIrrelevantInheritedFlags(cmd *cobra.Command) func() {
	if !cmd.HasParent() || cmd.Name() == "help" {
		return func() {}
	}
	path := cmd.CommandPath()
	var prev []struct {
		flag   *pflag.Flag
		hidden bool
	}
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		owners, scoped := pathFlagOwners[f.Name]
		if !scoped {
			return
		}
		if owners[path] {
			return
		}
		prev = append(prev, struct {
			flag   *pflag.Flag
			hidden bool
		}{f, f.Hidden})
		f.Hidden = true
	})
	return func() {
		for _, p := range prev {
			p.flag.Hidden = p.hidden
		}
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if errors.Is(err, errUserAborted) {
			ui.Skipf(os.Stderr, "cancelled")
			os.Exit(1)
		}
		ui.Errf(os.Stderr, "%s", err)
		var h Hinter
		if errors.As(err, &h) {
			ui.Hintf(os.Stderr, "%s", h.Hint())
		}
		var ue *usageError
		if errors.As(err, &ue) {
			os.Exit(2)
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
	home         string
	configDir    string
	binDir       string
	generatedDir string
	dryRun       bool
	force        bool
	logLevel     string
	quiet        bool
	debug        bool
	log          *chlog.Logger

	// Provenance recorded by resolvePaths for the first-run guard:
	// filesFromCwd is true when cfg.files fell through every explicit tier
	// (flag, $DOTD_FILES, $DOTFILES, config.yaml) to the cwd default;
	// configExists is whether config.yaml was present at resolve time.
	filesFromCwd bool
	configExists bool

	// configWarning is set by resolvePaths when config.yaml was present but
	// could not be parsed strictly (legacy/unknown fields or malformed YAML).
	// Resolution degrades gracefully; the warning is emitted once, after the
	// logger is configured.
	configWarning string
}

// config is a type alias so sub-command constructors can use the short name.
type config = appConfig

func newRootCmd() *cobra.Command {
	cfg := &config{}

	root := &cobra.Command{
		Use:   ecosystem.ToolD,
		Short: "Dotfiles manager — env resolution, DAG, symlinks, and init.sh generation",
		Long: `dotd — dotfiles manager: env resolution, DAG, symlinks, and init.sh generation.

Run 'dotd docs --full' for the complete machine-readable reference (concepts,
reference docs, and the full CLI help) embedded in the binary — intended for
agents and tooling.`,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&cfg.files, "files", "f", "", fmt.Sprintf("path to dotfiles repo (default: $DOTD_FILES → $DOTFILES → %s dotfiles → cwd)", ecosystem.ConfigFileName))
	pf.StringVar(&cfg.configPath, "dotd-config", "", fmt.Sprintf("path to dot-dagger's own %[1]s (default: $DOTD_CONFIG_FILE → ~/.config/dot-dagger/%[1]s)", ecosystem.ConfigFileName))
	pf.StringVar(&cfg.envFile, "dotd-env", "", fmt.Sprintf("path to dot-dagger's own %s (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/%s)", ecosystem.EnvFileName, ecosystem.EnvFileName))
	pf.StringArrayVar(&cfg.env, "env", nil, "env override as key=value (repeatable)")
	pf.BoolVar(&cfg.dryRun, "dry-run", false, "print actions without executing")
	pf.BoolVar(&cfg.force, "force", false, "override safety checks")
	pf.StringVar(&cfg.logLevel, "log-level", "info", "log verbosity ("+dotlog.LevelNames()+")")
	pf.BoolVar(&cfg.quiet, "quiet", false, "suppress informational logs (data output is unaffected)")
	pf.BoolVar(&cfg.debug, "debug", false, "shorthand for --log-level=debug")

	// Flag parse errors (unknown flag, bad value, etc.) are usage errors —
	// wrap them so main() can exit 2 instead of 1.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return asUsageError(err)
	})

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := resolvePaths(cfg); err != nil {
			return err
		}
		if err := configureLogger(cfg, cmd); err != nil {
			return err
		}
		if cfg.configWarning != "" {
			ui.Warnf(cmd.ErrOrStderr(), "%s", cfg.configWarning)
		}
		return nil
	}

	// 'dotd help --all' reveals hidden internal commands. The flag lives on
	// the help command itself (git-style), not on the root persistent set —
	// a persistent --all would collide with 'unapply --all', which means
	// something unrelated.
	var helpAll bool
	helpCmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long:  fmt.Sprintf("Help provides help for any command in the application.\nSimply type %s help [path to command] for full details.", ecosystem.ToolD),
		Run: func(c *cobra.Command, args []string) {
			target, _, err := c.Root().Find(args)
			if target == nil || err != nil {
				c.Printf("Unknown help topic %#q\n", args)
				_ = c.Root().Usage()
				return
			}
			target.InitDefaultHelpFlag()
			_ = target.Help()
		},
	}
	helpCmd.Flags().BoolVar(&helpAll, "all", false, "show all commands including internal helpers")
	root.SetHelpCommand(helpCmd)
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if helpAll {
			for _, sub := range cmd.Root().Commands() {
				sub.Hidden = false
			}
		}
		restore := hideIrrelevantInheritedFlags(cmd)
		defer restore()
		// Mirror cobra's default Help: print Long (or Short) then Usage.
		// cmd.Usage() alone uses UsageTemplate which omits the Long block.
		if cmd.Long != "" {
			cmd.Println(cmd.Long)
			cmd.Println()
		} else if cmd.Short != "" {
			cmd.Println(cmd.Short)
			cmd.Println()
		}
		_ = cmd.Usage()
	})

	ui.SetupCobraColors(root)

	root.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "advanced", Title: "Advanced:"},
		&cobra.Group{ID: "reference", Title: "Reference:"},
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
		newPathsCmd(cfg),
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

	conceptsCmd := newConceptsCmd()
	conceptsCmd.GroupID = "reference"
	docsCmd := newDocsCmd()
	docsCmd.GroupID = "reference"
	completionCmd := newCompletionCmd()
	completionCmd.GroupID = "reference"
	root.AddCommand(completionCmd)
	root.AddCommand(conceptsCmd)
	root.AddCommand(docsCmd)

	return root
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate shell completion script",
		Long: `Print a shell completion script to stdout for the chosen shell.

Source the script (or save it to your shell's completions dir) to enable
tab-completion for dotd commands and flags.

Examples:
  # bash — append to ~/.bashrc
  dotd completion bash >> ~/.bashrc

  # zsh — write to a completions dir
  dotd completion zsh > "${fpath[1]}/_dotd"

  # fish
  dotd completion fish > ~/.config/fish/completions/dotd.fish

  # powershell
  dotd completion powershell | Out-String | Invoke-Expression`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      usageArgs(cobra.ExactArgs(1)),
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
				return &usageError{err: fmt.Errorf("unsupported shell %q — choose bash, zsh, fish, or powershell", args[0])}
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
//
// Full resolution chain (CLI flag > DOTD_* env var > config.yaml field > XDG/system default)
// applies only to envFile, configPath, and files.
//
// The XDG-derived fields (home, binDir, configDir, generatedDir, initFile) are read
// directly from ecosystem accessors — they have no user-override tier.
func resolvePaths(cfg *config) error {
	var err error

	// env-file first — no config.yaml lookup (would be circular).
	cfg.envFile, err = ecosystem.ResolvePath(cfg.envFile, "DOTD_ENV_FILE", "", ecosystem.DefaultEnvFile)
	if err != nil {
		return err
	}

	// Tool preferences from config.yaml. Path stored in cfg so config subcommands don't re-resolve it.
	// No config-file value tier (third arg "") — config.yaml is what we're resolving; would be circular.
	cfg.configPath, err = ecosystem.ResolvePath(cfg.configPath, "DOTD_CONFIG_FILE", "", ecosystem.DefaultConfigFile)
	if err != nil {
		return err
	}
	// Lenient load: the preamble only needs `dotfiles`, so a legacy or corrupt
	// config.yaml must not hard-fail every command (including teardown, whose
	// job is to remove it). Degrade gracefully and warn; strict validation lives
	// in the `config` subcommands.
	toolCfg, unknownFields, err := dotcfg.LoadLenient(cfg.configPath)
	if err != nil {
		toolCfg = &dotcfg.Config{}
		cfg.configWarning = fmt.Sprintf("%s could not be parsed (%v); ignoring it — run 'dotd setup' to rewrite it", ecosystem.ConfigFileName, err)
	} else if len(unknownFields) > 0 {
		cfg.configWarning = fmt.Sprintf("%s has unrecognized field(s): %s (ignored) — run 'dotd setup' to rewrite it", ecosystem.ConfigFileName, strings.Join(unknownFields, ", "))
	}
	_, statErr := os.Stat(cfg.configPath)
	cfg.configExists = statErr == nil

	// Record provenance before resolving: when every explicit tier is empty
	// the default falls through to the cwd ($DOTFILES is checked here too
	// because DefaultDotfiles consults it inside the default tier).
	cfg.filesFromCwd = cfg.files == "" && os.Getenv("DOTD_FILES") == "" &&
		os.Getenv("DOTFILES") == "" && toolCfg.Dotfiles == ""

	cfg.files, err = ecosystem.ResolvePath(cfg.files, "DOTD_FILES", toolCfg.Dotfiles, ecosystem.DefaultDotfiles)
	if err != nil {
		return err
	}

	if cfg.home, err = ecosystem.Home(); err != nil {
		return err
	}
	if cfg.binDir, err = ecosystem.BinDir(); err != nil {
		return err
	}
	if cfg.configDir, err = ecosystem.XdgConfigHome(); err != nil {
		return err
	}
	if cfg.generatedDir, err = ecosystem.GeneratedDir(); err != nil {
		return err
	}
	if cfg.initFile, err = ecosystem.InitFile(); err != nil {
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
// cfg.home is guaranteed non-empty after resolvePaths succeeds.
func buildActOptions(cfg *config, dryRun bool) pipeline.ActOptions {
	return pipeline.ActOptions{
		HomeDir:      cfg.home,
		BinDir:       cfg.binDir,
		ConfigDir:    cfg.configDir,
		GeneratedDir: cfg.generatedDir,
		DryRun:       dryRun || cfg.dryRun,
		Force:        cfg.force,
	}
}

type pipelineRun struct {
	resolvedCount int
	allCount      int
	activeCount   int
	result        *pipeline.ActResult
}

// runOpts tunes a single pipeline run.
//
//   - dryRun: pass through to Act so it plans links without touching the disk.
//   - interactive: when true (and stdin is a TTY), the filter stage prompts for
//     any missing @when keys instead of erroring. This is a write-path
//     affordance — only 'apply' sets it. check/unapply/teardown and every
//     read-only command leave it false, so a missing key surfaces as an
//     annotated hint error rather than ambushing the user with a form mid-op.
type runOpts struct {
	dryRun      bool
	interactive bool
}

// guardWalkSource refuses to walk the cwd-fallback dotfiles path on an
// unconfigured machine. Without it, a fresh-machine 'dotd apply' silently
// walks whatever directory the user happens to be in. The guard fires only
// when no explicit source was given AND config.yaml is absent, so configured
// setups (including an intentionally empty 'dotfiles' key) are unaffected.
func guardWalkSource(cfg *config) error {
	if cfg.filesFromCwd && !cfg.configExists {
		return &hintError{
			err:  errors.New("no dotfiles repo configured (refusing to walk the current directory)"),
			hint: "run 'dotd setup', or pass -f <path> / set $DOTFILES",
		}
	}
	return nil
}

// walkActive runs the shared pipeline preamble:
//
//	guard → resolveEnv → Walk → ValidateNodes(all) → filterWithPrompt → Order
//
// Validation covers every walked node (not just active ones) in both the
// read and write paths, so a config that apply rejects also fails under
// list/dag/bundle/compose/package — including nodes currently filtered out.
//
// interactive gates the missing-@when-key prompt: true only on the write path
// ('apply'). When false, a missing key flows through filterWithPrompt's
// non-TTY branch and returns an annotated hint error.
func walkActive(cmd *cobra.Command, cfg *config, interactive bool) (ordered []pipeline.RawNode, resolvedCount, allCount int, err error) {
	if err := guardWalkSource(cfg); err != nil {
		return nil, 0, 0, err
	}
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return nil, 0, 0, annotateKeyError(err)
	}

	nodes, disabled, err := pipeline.Walk(cfg.files)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("walk %s: %w", cfg.files, err)
	}
	for _, p := range disabled {
		cfg.log.Debugf("disabled: %s", p)
	}

	if err := pipeline.ValidateNodes(nodes, pipeline.ActOptions{HomeDir: cfg.home, BinDir: cfg.binDir, ConfigDir: cfg.configDir}); err != nil {
		return nil, 0, 0, err
	}

	// Registry backs installed()/installable() in @when predicates — the same
	// packages.yaml the package subcommands consult (empty when absent).
	reg, err := loadRegistry(cfg)
	if err != nil {
		return nil, 0, 0, err
	}

	active, err := filterWithPrompt(cmd, nodes, resolved, interactive && isTTY(cmd.InOrStdin()), reg)
	if err != nil {
		return nil, 0, 0, err
	}

	ordered, err = pipeline.Order(active)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("order: %w", err)
	}
	return ordered, len(resolved), len(nodes), nil
}

func runPipeline(cmd *cobra.Command, cfg *config, opts runOpts) (*pipelineRun, error) {
	ordered, resolvedCount, allCount, err := walkActive(cmd, cfg, opts.interactive)
	if err != nil {
		return nil, err
	}

	actOpts := buildActOptions(cfg, opts.dryRun)
	res, err := pipeline.Act(ordered, actOpts)
	if err != nil {
		return nil, fmt.Errorf("act: %w", err)
	}

	return &pipelineRun{
		resolvedCount: resolvedCount,
		allCount:      allCount,
		activeCount:   len(ordered),
		result:        res,
	}, nil
}

// walkOrdered is walkActive for read-only commands that need just the
// ordered active nodes. Write-path commands use runPipeline (walkActive + Act).
// Read-only commands never prompt — a missing @when key is a hint error.
func (cfg *appConfig) walkOrdered(cmd *cobra.Command) ([]pipeline.RawNode, error) {
	ordered, _, _, err := walkActive(cmd, cfg, false)
	return ordered, err
}

func annotateKeyError(err error) error {
	var mke *predicate.MissingKeyError
	if errors.As(err, &mke) {
		return &hintError{
			err:  err,
			hint: fmt.Sprintf("set it with --env %s=<value> or add it to %s", mke.Key, ecosystem.EnvFileName),
		}
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

apply is idempotent and resumable, not transactional. If a run fails partway
(e.g. a destination is blocked by a non-symlink file), the work done so far
stays on disk; fix the cause and re-run 'dotd apply' to converge to the full
plan — there is no rollback and no manual cleanup needed.

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
	if guardWalkSource(cfg) != nil {
		return // unconfigured cwd fallback — the pipeline will refuse anyway
	}
	_ = filepath.WalkDir(cfg.files, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if fileutil.IsGitDir(d) {
			return filepath.SkipDir
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, node.PrefixNoSync) {
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
	// apply is the one interactive write path: prompt for missing @when keys.
	run, err := runPipeline(cmd, cfg, runOpts{interactive: true})
	if err != nil {
		return err
	}
	cfg.log.Debugf("%s %d keys", ui.Header("env:"), run.resolvedCount)
	cfg.log.Debugf("%s %d active / %d total", ui.Header("filter:"), run.activeCount, run.allCount)

	if run.activeCount > 0 &&
		len(run.result.Links)+len(run.result.Sourced)+len(run.result.Generated) == 0 {
		cfg.log.Warnf("%s produced no actions — convention dirs may be missing .dagger files; run 'dotd init'",
			plural(run.activeCount, "active node"))
	}

	if cfg.dryRun {
		for _, lnk := range run.result.Links {
			fmt.Fprintf(cmd.OutOrStdout(), "dry-run: link %s %s %s\n", lnk.Src, ui.Arrow("→"), lnk.Dest)
		}
		for _, gen := range run.result.Generated {
			if gen.Path == "" {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would write %s (compose target %s)\n", gen.Path, gen.Node.LogicalName)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would write %s (%s sourced)\n", cfg.initFile, plural(len(run.result.Sourced), "node"))
		return nil
	}

	// Mutation results are user-facing output, not log diagnostics — they go
	// to stdout and are not suppressed by --quiet (channel policy, 2026-06-13
	// audit O1). Stage detail stays on cfg.log above.
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s applied\n", ui.Header("links:"), plural(len(run.result.Links), "symlink"))

	if err := pipeline.Generate(cfg.initFile, run.result.Sourced, pipeline.GenerateOptions{
		BinDir: cfg.binDir,
	}); err != nil {
		return fmt.Errorf("generate init.sh: %w", err)
	}
	fmt.Fprintf(out, "%s wrote %s (%s)\n", ui.Header("init.sh:"), cfg.initFile, plural(len(run.result.Sourced), "node"))
	return nil
}

func runCheck(cmd *cobra.Command, cfg *config) error {
	warnIfNosyncUnignored(cfg)
	// check is a preview, not a write: report a missing @when key, don't prompt.
	run, err := runPipeline(cmd, cfg, runOpts{dryRun: true})
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
		cfg.log.Infof("%s %s present (%s sourced)", ui.Header("init.sh:"), cfg.initFile, plural(len(run.result.Sourced), "node"))
	}

	if hasIssues {
		cmd.SilenceErrors = true
		if !cfg.configExists {
			return &hintError{
				err:  errors.New("check: issues found"),
				hint: "no config.yaml found — run 'dotd setup' to configure this machine",
			}
		}
		return &hintError{
			err:  errors.New("check: issues found"),
			hint: "run 'dotd apply' to create missing and repair wrong symlinks — add --force to overwrite a non-symlink file at a destination",
		}
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
