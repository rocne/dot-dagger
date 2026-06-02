package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard: configure dot-dagger at the system level",
		Long: fmt.Sprintf(`Configure dot-dagger for this machine.

Writes config.yaml and (if absent) %s to the platform config dir.
If config.yaml already exists, current values are shown as defaults.

Does not create symlinks or scaffold .dagger files.
Run 'dotd init' next to scaffold your dotfiles repo.`, ecosystem.EnvFileName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, cfg)
		},
	}
}

func runSetup(cmd *cobra.Command, cfg *config) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())
	// home is used only to expand "~" in user-typed paths, not for config resolution.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// cfg.configPath is resolved by PersistentPreRunE.
	// Load existing config.yaml — returns empty Config (no error) if absent.
	existing, err := dotcfg.Load(cfg.configPath)
	if err != nil {
		return fmt.Errorf("setup: load existing config: %w", err)
	}

	isUpdate := existing.Dotfiles != "" || existing.BinDir != "" || existing.GeneratedDir != "" || existing.LinkRoot != ""
	if isUpdate {
		fmt.Fprintf(out, "%s — update machine configuration\n", ui.Header("dotd setup"))
		fmt.Fprintln(out, "Press Enter to keep the current value.")
	} else {
		fmt.Fprintf(out, "%s — configure this machine\n", ui.Header("dotd setup"))
		fmt.Fprintln(out, "Press Enter to accept the shown default.")
	}

	// Dotfiles path. Use cfg.files — already resolved by resolvePaths.
	dotfilesDefault := existing.Dotfiles
	if dotfilesDefault == "" {
		dotfilesDefault = cfg.files
	}
	printField(out, "Dotfiles repo", "Your dotfiles git repository.")
	dotfilesPath, err := promptDefault(out, reader, fieldPrompt(), dotfilesDefault, false)
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
	printField(out, "Bin directory", "Where executable scripts from your dotfiles repo are linked.")
	binDir, err := promptDefault(out, reader, fieldPrompt(), binDirDefault, false)
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
	printField(out, "Generated files directory", "Where compose-assembled shell fragments are written.")
	generatedDir, err := promptDefault(out, reader, fieldPrompt(), generatedDirDefault, false)
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
	printField(out, "Link root", "Home directory used for ~ expansion in link destinations (default: $HOME).")
	linkRoot, err := promptDefault(out, reader, fieldPrompt(), linkRootDefault, false)
	if err != nil {
		return err
	}
	linkRoot = expandTildeStr(linkRoot, home)
	linkRoot, err = filepath.Abs(linkRoot)
	if err != nil {
		return err
	}

	fmt.Fprintln(out)

	// Write config.yaml.
	toolCfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		LinkRoot:     linkRoot,
	}
	if err := dotcfg.Save(cfg.configPath, toolCfg); err != nil {
		return fmt.Errorf("setup: save config.yaml: %w", err)
	}
	ui.OKf(out, "  wrote %s", cfg.configPath)

	// Write env.yaml only if absent. cfg.envFile is already resolved by resolvePaths.
	envPath := cfg.envFile
	if _, err := os.Stat(envPath); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
			return fmt.Errorf("setup: mkdir %s: %w", filepath.Dir(envPath), err)
		}
		envContent := fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)
		if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
			return fmt.Errorf("setup: write %s: %w", ecosystem.EnvFileName, err)
		}
		ui.OKf(out, "  wrote %s", envPath)
	} else {
		ui.Skipf(out, "  exists %s", envPath)
	}

	ui.Headerf(out, "Next steps:")
	fmt.Fprintf(out, "  1. %s   scaffold convention directories in your dotfiles repo\n", ui.Key("dotd init"))
	fmt.Fprintln(out, "  2. Add dotfiles to your repo")
	fmt.Fprintf(out, "  3. %s\n", ui.Key("dotd apply"))
	return nil
}

// printField prints a bold field label and a faint description, then a blank line.
func printField(w io.Writer, label, desc string) {
	fmt.Fprintf(w, "\n  %s\n", ui.Key(label))
	fmt.Fprintf(w, "  %s\n", ui.Skip(desc))
}

// fieldPrompt returns the prompt text used after a printField call.
func fieldPrompt() string {
	return "  " + ui.Arrow("›")
}
