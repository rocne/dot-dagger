package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newSetupCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard: configure dot-dagger at the system level",
		Long: `Configure dot-dagger for this machine.

Writes config.yaml and (if absent) env.yaml to the platform config dir.
If config.yaml already exists, current values are shown as defaults.

Does not create symlinks or scaffold .dagger files.
Run 'dotd init' next to scaffold your dotfiles repo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, cfg)
		},
	}
}

func runSetup(cmd *cobra.Command, cfg *config) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	// home is used only to expand "~" in user-typed paths, not for config resolution.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Load existing config.yaml — returns empty Config (no error) if absent.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	existing, err := dotcfg.Load(configPath)
	if err != nil {
		return fmt.Errorf("setup: load existing config: %w", err)
	}

	isUpdate := existing.Dotfiles != "" || existing.BinDir != "" || existing.GeneratedDir != "" || existing.LinkRoot != ""
	if isUpdate {
		fmt.Fprintln(cmd.OutOrStdout(), "Updating dot-dagger configuration — press Enter to keep current value.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "dot-dagger setup wizard — press Enter to accept defaults.")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Dotfiles path. Use cfg.files — already resolved by resolvePaths (DOTD_FILES → DOTFILES → config → default).
	dotfilesDefault := existing.Dotfiles
	if dotfilesDefault == "" {
		dotfilesDefault = cfg.files
	}
	dotfilesPath, err := promptDefault(cmd.OutOrStdout(), reader, "Dotfiles repo (your dotfiles git repository)", dotfilesDefault, false)
	if err != nil {
		return err
	}
	dotfilesPath = expandTildeStr(dotfilesPath, home)
	dotfilesPath, err = filepath.Abs(dotfilesPath)
	if err != nil {
		return err
	}

	// Bin dir. Use cfg.binDir — already resolved by resolvePaths.
	binDirDefault := existing.BinDir
	if binDirDefault == "" {
		binDirDefault = cfg.binDir
	}
	binDir, err := promptDefault(cmd.OutOrStdout(), reader, "Bin directory (generated shell wrappers)", binDirDefault, false)
	if err != nil {
		return err
	}
	binDir = expandTildeStr(binDir, home)
	binDir, err = filepath.Abs(binDir)
	if err != nil {
		return err
	}

	// Generated dir. Use cfg.generatedDir — already resolved by resolvePaths.
	generatedDirDefault := existing.GeneratedDir
	if generatedDirDefault == "" {
		generatedDirDefault = cfg.generatedDir
	}
	generatedDir, err := promptDefault(cmd.OutOrStdout(), reader, "Generated files directory (assembled compose targets)", generatedDirDefault, false)
	if err != nil {
		return err
	}
	generatedDir = expandTildeStr(generatedDir, home)
	generatedDir, err = filepath.Abs(generatedDir)
	if err != nil {
		return err
	}

	// Link root. Use cfg.linkRoot — already resolved by resolvePaths.
	linkRootDefault := existing.LinkRoot
	if linkRootDefault == "" {
		linkRootDefault = cfg.linkRoot
	}
	linkRoot, err := promptDefault(cmd.OutOrStdout(), reader, "Symlink root (where dotfiles symlink to, usually $HOME)", linkRootDefault, false)
	if err != nil {
		return err
	}
	linkRoot = expandTildeStr(linkRoot, home)
	linkRoot, err = filepath.Abs(linkRoot)
	if err != nil {
		return err
	}

	// Write config.yaml.
	toolCfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		LinkRoot:     linkRoot,
	}
	if err := dotcfg.Save(configPath, toolCfg); err != nil {
		return fmt.Errorf("setup: save config.yaml: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)

	// Write env.yaml only if absent.
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(envPath); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
			return fmt.Errorf("setup: mkdir %s: %w", filepath.Dir(envPath), err)
		}
		envContent := fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)
		if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
			return fmt.Errorf("setup: write env.yaml: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", envPath)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "exists %s (not modified)\n", envPath)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Run: %s init  (scaffold .dagger convention files)\n", ecosystem.ToolD)
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Add dotfiles to your repo")
	fmt.Fprintf(cmd.OutOrStdout(), "  3. Run: %s apply\n", ecosystem.ToolD)
	return nil
}
