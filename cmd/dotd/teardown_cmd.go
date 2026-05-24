package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/rocne/dot-dagger/internal/setup"
	"github.com/spf13/cobra"
)

func newTeardownCmd(cfg *config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "teardown",
		Short: "Remove dot-dagger system config (config.yaml, env.yaml, RC source line)",
		Long: `Remove dot-dagger system-level configuration from this machine.

Removes:
  - config.yaml from the platform config dir
  - env.yaml from the platform config dir
  - The dotd source line from the shell RC file (if detected)

Does NOT remove symlinks or .dagger files.
Run 'dotd unapply' first to remove symlinks, then 'dotd teardown'.

Shows a preview and prompts for confirmation before making any changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeardown(cmd, cfg, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runTeardown(cmd *cobra.Command, cfg *config, yes bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	// Pre-action check: warn if active symlinks detected.
	// Non-fatal if walk fails (env.yaml or dotfiles repo may be absent).
	if prun, err := runPipeline(cfg, true); err == nil {
		if len(prun.result.Links) > 0 {
			fmt.Fprintf(out, "warning: %d symlink(s) still active — consider running 'dotd unapply' first\n", len(prun.result.Links))
		}
	}

	// Pre-action check: warn if .dagger files still present.
	if cfg.files != "" && hasDaggerFiles(cfg.files) {
		fmt.Fprintln(out, "warning: .dagger files present in dotfiles repo — these will not be removed")
	}

	// Determine paths. Both call DefaultPath() directly — teardown removes the
	// system-level files regardless of any --env-file / --files flag overrides.
	// This is a deliberate exception to the canonical resolution rule.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}

	// Determine RC file path — requires env.yaml to know the shell.
	// If resolveEnv fails (env.yaml absent etc.), RC stripping is skipped.
	rcFile := ""
	if resolved, rerr := resolveEnv(cfg); rerr == nil {
		if shell := resolved["shell"]; shell != "" {
			osName := resolved["os"]
			if sc, ok, _ := setup.DetectShellConfig(shell, osName, cfg.linkRoot); ok {
				has, _ := setup.HasSourceLine(sc.RCFile, cfg.initFile)
				if has {
					rcFile = sc.RCFile
				}
			}
		}
	}

	// Stat each file to determine what exists.
	configExists := fileExists(configPath)
	envExists := fileExists(envPath)

	// Preview.
	fmt.Fprintln(out, "\nWill remove:")
	if configExists {
		fmt.Fprintf(out, "  %s\n", configPath)
	} else {
		fmt.Fprintf(out, "  %s (not found, skip)\n", configPath)
	}
	if envExists {
		fmt.Fprintf(out, "  %s\n", envPath)
	} else {
		fmt.Fprintf(out, "  %s (not found, skip)\n", envPath)
	}
	if rcFile != "" {
		fmt.Fprintf(out, "  source line from %s\n", rcFile)
	} else {
		fmt.Fprintln(out, "  RC source line (not detected, skip)")
	}

	// Confirmation.
	if !yes {
		fmt.Fprint(out, "\nProceed? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "cancelled")
			return nil
		}
	}

	// Execute.
	if configExists {
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("teardown: remove %s: %w", configPath, err)
		}
		fmt.Fprintf(out, "removed %s\n", configPath)
	} else {
		fmt.Fprintf(out, "not found, skip: %s\n", configPath)
	}

	if envExists {
		if err := os.Remove(envPath); err != nil {
			return fmt.Errorf("teardown: remove %s: %w", envPath, err)
		}
		fmt.Fprintf(out, "removed %s\n", envPath)
	} else {
		fmt.Fprintf(out, "not found, skip: %s\n", envPath)
	}

	if rcFile != "" {
		if err := setup.RemoveSourceLine(rcFile, cfg.initFile); err != nil {
			return fmt.Errorf("teardown: strip RC source line: %w", err)
		}
		fmt.Fprintf(out, "removed source line from %s\n", rcFile)
	}

	// Prune config dir if now empty.
	configDir := filepath.Dir(configPath)
	if entries, err := os.ReadDir(configDir); err == nil && len(entries) == 0 {
		if err := os.Remove(configDir); err == nil {
			fmt.Fprintf(out, "removed %s (empty)\n", configDir)
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
