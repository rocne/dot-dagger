package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/setup"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd(rootCfg *config) *cobra.Command {
	var (
		yes         bool
		interactive bool // explicit flag; default behavior, useful for alias overrides
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive onboarding: scaffold dotfiles repo structure and config files",
		Long: `Walk through creating a dotfiles repo structure,
writing env.yaml, and wiring up your shell init file.

Run without flags for interactive mode (default). Use --yes to skip all
prompts and accept defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = interactive // explicit flag, no-op beyond enabling default behavior
			return runSetup(cmd, rootCfg, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: accept all defaults")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "interactive mode (default; explicit override for aliases)")
	return cmd
}

func runSetup(cmd *cobra.Command, rootCfg *config, nonInteractive bool) error {
	if !nonInteractive && !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf(ecosystem.ToolD + " setup: no TTY detected; run with --yes to accept defaults")
	}

	// Detect basic environment for pre-populating prompts.
	detectedOS := runtime.GOOS
	if detectedOS == "darwin" {
		detectedOS = "macos"
	}
	hostname, _ := os.Hostname()
	detected := map[string]string{
		"os":       detectedOS,
		"hostname": hostname,
	}

	// Defaults — shown as pre-filled values in interactive mode.
	// All paths are already resolved by PersistentPreRunE.
	dotfilesDir := rootCfg.files
	envFile := rootCfg.envFile
	initFile := rootCfg.initFile

	// Detect installed package managers for pre-selection.
	installedMgrs := packages.DetectInstalled()
	installedSet := make(map[string]bool, len(installedMgrs))
	for _, m := range installedMgrs {
		installedSet[m] = true
	}

	// Build multi-select options: installed managers first, rest below.
	var mgrOptions []huh.Option[string]
	for _, m := range packages.Catalog {
		var label string
		if installedSet[m.Name] {
			label = m.Name + "  (" + m.Description + ", installed)"
		} else {
			label = m.Name + "  (" + m.Description + ")"
		}
		mgrOptions = append(mgrOptions, huh.NewOption(label, m.Name).Selected(installedSet[m.Name]))
	}

	// Sort: installed first, preserving catalog order within each group.
	var installedOpts, otherOpts []huh.Option[string]
	for _, opt := range mgrOptions {
		if installedSet[opt.Value] {
			installedOpts = append(installedOpts, opt)
		} else {
			otherOpts = append(otherOpts, opt)
		}
	}
	mgrOptions = append(installedOpts, otherOpts...)

	var selectedMgrs []string
	if nonInteractive {
		selectedMgrs = installedMgrs
	} else {
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Dotfiles directory").
					Value(&dotfilesDir),
				huh.NewInput().
					Title("env.yaml location").
					Value(&envFile),
				huh.NewInput().
					Title("init.sh output").
					Value(&initFile),
			),
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Package managers to pre-fill in packages.yaml").
					Description("Installed managers are pre-selected. Select any others you want included.").
					Options(mgrOptions...).
					Value(&selectedMgrs),
			),
		).Run(); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
		var err error
		if dotfilesDir, err = expandHome(dotfilesDir); err != nil {
			return err
		}
		if envFile, err = expandHome(envFile); err != nil {
			return err
		}
		if initFile, err = expandHome(initFile); err != nil {
			return err
		}
	}

	// Scaffold repo structure and config files.
	result, err := setup.Scaffold(setup.Options{
		DotfilesDir:      dotfilesDir,
		EnvFilePath:      envFile,
		InitFilePath:     initFile,
		DetectedOS:       detected["os"],
		DetectedDistro:   detected["distro"],
		DetectedShell:    detected["shell"],
		SelectedManagers: selectedMgrs,
	})
	if err != nil {
		return err
	}

	// Print actions.
	for _, act := range result.Actions {
		var state string
		if act.State == setup.StateCreated {
			state = ui.OK("created")
		} else {
			state = ui.Skip("exists ")
		}
		rootCfg.log.Infof("  %s  %s", state, act.Path)
	}

	// Shell hook.
	if err := handleShellHook(rootCfg, detected, initFile, nonInteractive); err != nil {
		return err
	}

	printNextSteps(rootCfg, dotfilesDir, initFile)
	return nil
}

func handleShellHook(cfg *config, detected map[string]string, initFile string, nonInteractive bool) error {
	shell := detected["shell"]
	osName := detected["os"]

	sc, ok, err := setup.DetectShellConfig(shell, osName)
	if err != nil {
		return err
	}
	if !ok {
		sourceLine, err := setup.SourceLine(initFile)
		if err != nil {
			return err
		}
		cfg.log.Warnf("%s unknown shell %q — add manually:\n  %s", ui.Header("shell:"), shell, sourceLine)
		return nil
	}

	// For bash on macOS, the right RC file is ambiguous — prompt to confirm.
	if !nonInteractive && shell == "bash" && osName == "macos" {
		rcFile := sc.RCFile
		if err := huh.NewInput().
			Title("bash RC file").
			Value(&rcFile).
			Run(); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
		sc.RCFile, err = expandHome(rcFile)
		if err != nil {
			return err
		}
	}

	has, err := setup.HasSourceLine(sc.RCFile, initFile)
	if err != nil {
		return err
	}
	if has {
		cfg.log.Infof("%s %s already configured", ui.Header("shell:"), sc.RCFile)
		return nil
	}

	sourceLine, err := setup.SourceLine(initFile)
	if err != nil {
		return err
	}
	cfg.log.Infof("%s add to %s:\n\n  echo '%s' >> %s\n", ui.Header("shell:"), sc.RCFile, sourceLine, sc.RCFile)

	if nonInteractive {
		return nil
	}

	var doAppend bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Append source line to %s?", sc.RCFile)).
		Affirmative("Yes").
		Negative("No").
		Value(&doAppend).
		Run(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	if doAppend {
		if err := setup.AppendSourceLine(sc.RCFile, initFile); err != nil {
			return err
		}
		cfg.log.Infof("  %s appended to %s", ui.OK("ok"), sc.RCFile)
	}
	return nil
}

func printNextSteps(cfg *config, dotfilesDir, initFile string) {
	sourceLine, _ := setup.SourceLine(initFile) // failure means home dir unknown; show raw path
	if sourceLine == "" {
		sourceLine = initFile
	}
	cfg.log.Infof("%s", ui.Header("next steps:"))
	cfg.log.Infof("  1. Add dotfiles to %s", dotfilesDir)
	cfg.log.Infof("  2. Run: %s check", ecosystem.ToolD)
	cfg.log.Infof("  3. Reload shell: %s", sourceLine)
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
