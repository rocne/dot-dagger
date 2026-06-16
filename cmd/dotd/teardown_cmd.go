package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/setup"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newTeardownCmd(cfg *config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "teardown",
		Short: fmt.Sprintf("Remove dot-dagger system config (config.yaml, %s, RC source line)", ecosystem.EnvFileName),
		Long: fmt.Sprintf(`Remove dot-dagger system-level configuration from this machine.

Removes (at the resolved paths — honors --dotd-config, --dotd-env, DOTD_* vars):
  - config.yaml
  - %s
  - The dotd source line from the shell RC file (if detected)

Does NOT remove symlinks or .dagger files.
Run 'dotd unapply' first to remove symlinks, then 'dotd teardown'.

Shows a preview and prompts for confirmation before making any changes.

Examples:
  dotd teardown            # interactive — shows preview, asks before removing
  dotd teardown --yes      # non-interactive (CI / scripts)`, ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeardown(cmd, cfg, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runTeardown(cmd *cobra.Command, cfg *config, yes bool) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	// Pre-action check: warn if active symlinks detected. Advisory only, so this
	// runs non-interactively — a missing @when key must not launch a prompt
	// before teardown's own preview; the walk just fails and the warning is
	// skipped. Non-fatal if walk fails (env.yaml or dotfiles repo may be absent).
	if prun, err := runPipeline(cmd, cfg, runOpts{dryRun: true}); err == nil {
		if len(prun.result.Links) > 0 {
			ui.Warnf(errOut, "%s still active — consider running 'dotd unapply' first", plural(len(prun.result.Links), "symlink"))
		}
	}

	// Pre-action check: warn if .dagger files still present.
	if cfg.files != "" && hasDaggerFiles(cfg.files) {
		ui.Warnf(errOut, ".dagger files present in dotfiles repo — these will not be removed")
	}

	// Teardown removes the files dotd is currently configured to use — the
	// same resolved paths every other command consumes (flag → DOTD_* →
	// default). The preview below shows the exact paths before anything is
	// touched.
	configPath := cfg.configPath
	envPath := cfg.envFile

	// Determine RC file path — requires env.yaml to know the shell.
	// If resolveEnv fails (env.yaml absent etc.), RC stripping is skipped.
	rcFile := ""
	if resolved, rerr := resolveEnv(cfg); rerr == nil {
		if shell := resolved["shell"]; shell != "" {
			osName := resolved["os"]
			if sc, ok := setup.DetectShellConfig(shell, osName, cfg.home, cfg.configDir); ok {
				has, _ := setup.HasSourceLine(sc.RCFile, cfg.initFile)
				if has {
					rcFile = sc.RCFile
				}
			}
		}
	}

	// Stat each file to determine what exists.
	configExists := fileutil.Exists(configPath)
	envExists := fileutil.Exists(envPath)

	// Preview.
	ui.Headerf(out, "Will remove:")
	if configExists {
		fmt.Fprintf(out, "  %s\n", configPath)
	} else {
		fmt.Fprintf(out, "  %s %s\n", configPath, ui.Skip("(not found, skip)"))
	}
	if envExists {
		fmt.Fprintf(out, "  %s\n", envPath)
	} else {
		fmt.Fprintf(out, "  %s %s\n", envPath, ui.Skip("(not found, skip)"))
	}
	if rcFile != "" {
		fmt.Fprintf(out, "  source line from %s\n", rcFile)
	} else {
		fmt.Fprintf(out, "  RC source line %s\n", ui.Skip("(not detected, skip)"))
	}

	// Dry-run stops here — preview printed, nothing removed.
	if cfg.dryRun {
		return nil
	}

	// Confirmation.
	if !yes && !promptConfirm(out, cmd.InOrStdin()) {
		return nil
	}

	// Execute best-effort: a failure on one target must not skip the others, so
	// teardown never leaves a half-removed install just because the first target
	// resisted. Failures are reported to stderr and aggregated into a non-zero
	// exit — the same contract runUnapply follows.
	var failures int
	if configExists {
		if err := os.Remove(configPath); err != nil {
			ui.Errf(errOut, "removing %s: %v", configPath, err)
			failures++
		} else {
			ui.OKf(out, "removed %s", configPath)
		}
	} else {
		ui.Skipf(out, "skip: %s", configPath)
	}

	if envExists {
		if err := os.Remove(envPath); err != nil {
			ui.Errf(errOut, "removing %s: %v", envPath, err)
			failures++
		} else {
			ui.OKf(out, "removed %s", envPath)
		}
	} else {
		ui.Skipf(out, "skip: %s", envPath)
	}

	if rcFile != "" {
		if err := setup.RemoveSourceLine(rcFile, cfg.initFile); err != nil {
			ui.Errf(errOut, "stripping RC source line from %s: %v", rcFile, err)
			failures++
		} else {
			ui.OKf(out, "removed source line from %s", rcFile)
		}
	}

	// Prune config dir if now empty.
	configDir := filepath.Dir(configPath)
	if entries, err := os.ReadDir(configDir); err == nil && len(entries) == 0 {
		if err := os.Remove(configDir); err == nil {
			ui.OKf(out, "removed %s (empty)", configDir)
		}
	}

	if failures > 0 {
		return fmt.Errorf("teardown: %s failed to remove", plural(failures, "target"))
	}
	return nil
}

// hasDaggerFiles reports whether any .dagger file exists under root.
func hasDaggerFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == ecosystem.ConfigFile {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
