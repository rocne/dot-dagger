package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/setup"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd(rootCfg *config) *cobra.Command {
	var (
		yes           bool
		noInteractive bool
		interactive   bool // explicit flag; default behavior, useful for alias overrides
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive onboarding: scaffold dotfiles repo structure and config files",
		Long: `scaffold walks you through creating a dotfiles repo structure,
writing env.yaml, and wiring up your shell init file.

Run without flags for interactive mode (default). Use --no-interactive or
--yes to skip all prompts and accept defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nonInteractive := yes || noInteractive
			_ = interactive // explicit flag, no-op beyond enabling default behavior
			return runSetup(cmd, rootCfg, nonInteractive)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: accept all defaults")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "non-interactive: accept all defaults")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "interactive mode (default; explicit override for aliases)")
	return cmd
}

func runSetup(cmd *cobra.Command, rootCfg *config, nonInteractive bool) error {
	if !nonInteractive && !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf(ecosystem.ToolD+" setup: no TTY detected; run with --no-interactive to accept defaults")
	}

	// Detect environment for pre-populating prompts and env.yaml comments.
	r := env.NewResolver()
	detected, _ := r.Resolve(nil)

	// Defaults — shown as pre-filled values in interactive mode.
	dotfilesDir := defaultDotfiles()
	envFile := rootCfg.envFile
	initFile := defaultInitFile()

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
	fmt.Fprintln(cmd.OutOrStdout())
	for _, act := range result.Actions {
		var state string
		if act.State == setup.StateCreated {
			state = ui.OK("created")
		} else {
			state = ui.Skip("exists ")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", state, act.Path)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Shell hook.
	if err := handleShellHook(cmd, detected, initFile, nonInteractive); err != nil {
		return err
	}

	printNextSteps(cmd, dotfilesDir, initFile)
	return nil
}

func handleShellHook(cmd *cobra.Command, detected map[string]string, initFile string, nonInteractive bool) error {
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s unknown shell %q — add manually:\n  %s\n\n",
			ui.Header("shell:"), shell, sourceLine)
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s already configured\n\n", ui.Header("shell:"), sc.RCFile)
		return nil
	}

	sourceLine, err := setup.SourceLine(initFile)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s add to %s:\n\n  echo '%s' >> %s\n\n",
		ui.Header("shell:"), sc.RCFile, sourceLine, sc.RCFile)

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
		fmt.Fprintf(cmd.OutOrStdout(), "  %s appended to %s\n", ui.OK("ok"), sc.RCFile)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

func printNextSteps(cmd *cobra.Command, dotfilesDir, initFile string) {
	sourceLine, _ := setup.SourceLine(initFile) // failure means home dir unknown; show raw path
	if sourceLine == "" {
		sourceLine = initFile
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", ui.Header("next steps:"))
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Add dotfiles to %s\n", dotfilesDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: %s check\n", ecosystem.ToolD)
	fmt.Fprintf(cmd.OutOrStdout(), "  3. Reload shell: %s\n", sourceLine)
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
