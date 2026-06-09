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
	"github.com/rocne/dot-dagger/internal/fileutil"
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
Run 'dotd init' next to scaffold your dotfiles repo.

Examples:
  dotd setup                              # first-time interactive setup
  dotd setup --config /tmp/custom.yaml    # write to a non-default path`, ecosystem.EnvFileName),
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

	dotfilesPath, err := promptPath(out, reader, "Dotfiles repo", "Your dotfiles git repository.", existing.Dotfiles, cfg.files, home)
	if err != nil {
		return err
	}

	binDir, err := promptPath(out, reader, "Bin directory", "Where executable scripts from your dotfiles repo are linked.", existing.BinDir, cfg.binDir, home)
	if err != nil {
		return err
	}

	generatedDir, err := promptPath(out, reader, "Generated files directory", "Where compose-assembled shell fragments are written.", existing.GeneratedDir, cfg.generatedDir, home)
	if err != nil {
		return err
	}

	linkRoot, err := promptPath(out, reader, "Link root", "Home directory used for ~ expansion in link destinations (default: $HOME).", existing.LinkRoot, cfg.linkRoot, home)
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
		if err := os.MkdirAll(filepath.Dir(envPath), fileutil.ModeDir); err != nil {
			return fmt.Errorf("setup: mkdir %s: %w", filepath.Dir(envPath), err)
		}
		envContent := fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)
		if err := os.WriteFile(envPath, []byte(envContent), fileutil.ModeFile); err != nil {
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

// promptPath prints a field prompt, reads a path from the user, expands ~ and resolves to absolute.
// existingVal takes precedence over resolvedDefault when non-empty (update vs first-run).
func promptPath(out io.Writer, reader *bufio.Reader, label, desc, existingVal, resolvedDefault, home string) (string, error) {
	def := existingVal
	if def == "" {
		def = resolvedDefault
	}
	printField(out, label, desc)
	val, err := promptDefault(out, reader, fieldPrompt(), def, false)
	if err != nil {
		return "", err
	}
	val = fileutil.ExpandHome(val, home)
	return filepath.Abs(val)
}

