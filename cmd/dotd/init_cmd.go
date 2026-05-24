package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

func newInitCmd(cfg *config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard: create config.yaml and env.yaml",
		Long: `Walk through creating or updating the tool configuration.

Writes ~/.config/dot-dagger/config.yaml and ~/.config/dot-dagger/env.yaml.
Optionally scaffolds .dagger convention files in your dotfiles repo.

Run without flags for interactive mode. Use --yes to accept all defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, cfg, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: accept all defaults")
	return cmd
}

func runInit(cmd *cobra.Command, cfg *config, nonInteractive bool) error {
	reader := bufio.NewReader(os.Stdin)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if !nonInteractive {
		fmt.Fprintln(cmd.OutOrStdout(), "Dotd setup wizard — press Enter to accept defaults.")
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Gather tool config values.
	dotfilesDefault := filepath.Join(home, "dotfiles")
	if d, ok := os.LookupEnv("DOTFILES"); ok {
		dotfilesDefault = d
	}

	dotfilesPath, err := promptDefault(cmd.OutOrStdout(), reader, "Dotfiles repo (your dotfiles git repository)", dotfilesDefault, nonInteractive)
	if err != nil {
		return err
	}
	dotfilesPath = expandTildeStr(dotfilesPath, home)
	dotfilesPath, err = filepath.Abs(dotfilesPath)
	if err != nil {
		return err
	}

	binDirDefault := filepath.Join(home, ".local", "bin", "dot-dagger")
	binDir, err := promptDefault(cmd.OutOrStdout(), reader, "Bin directory (generated shell wrappers)", binDirDefault, nonInteractive)
	if err != nil {
		return err
	}
	binDir = expandTildeStr(binDir, home)

	generatedDirDefault := filepath.Join(home, ".local", "share", "dot-dagger", "generated")
	generatedDir, err := promptDefault(cmd.OutOrStdout(), reader, "Generated files directory (assembled compose targets)", generatedDirDefault, nonInteractive)
	if err != nil {
		return err
	}
	generatedDir = expandTildeStr(generatedDir, home)

	linkRootDefault := home
	linkRoot, err := promptDefault(cmd.OutOrStdout(), reader, "Symlink root (where dotfiles symlink to, usually $HOME)", linkRootDefault, nonInteractive)
	if err != nil {
		return err
	}
	linkRoot = expandTildeStr(linkRoot, home)

	// Write config.yaml.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	toolCfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		LinkRoot:     linkRoot,
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", filepath.Dir(configPath), err)
	}
	if err := dotcfg.Save(configPath, toolCfg); err != nil {
		return fmt.Errorf("init: save config.yaml: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)

	// Write env.yaml if it doesn't exist.
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(envPath); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
			return fmt.Errorf("init: mkdir %s: %w", filepath.Dir(envPath), err)
		}
		envContent := fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)
		if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
			return fmt.Errorf("init: write env.yaml: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", envPath)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "exists %s (not modified)\n", envPath)
	}

	// Optionally scaffold .dagger files.
	if !nonInteractive {
		if err := scaffoldDaggerInteractive(reader, cmd, dotfilesPath); err != nil {
			return err
		}
	}

	// Wire shell RC file — append source line for init.sh if not already present.
	if err := maybeAddSourceLine(cmd, cfg, reader, nonInteractive); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Add dotfiles to %s\n", dotfilesPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: %s apply\n", ecosystem.ToolD)
	return nil
}

// maybeAddSourceLine checks whether the shell RC file already sources init.sh and
// offers to append the line if it does not.
func maybeAddSourceLine(cmd *cobra.Command, cfg *config, reader *bufio.Reader, nonInteractive bool) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		// env resolution failure is non-fatal here — init already wrote the files.
		fmt.Fprintf(cmd.OutOrStdout(), "note: could not resolve env to detect shell RC file: %v\n", err)
		return nil
	}

	shell := resolved["shell"]
	osName := resolved["os"]
	if shell == "" {
		return nil // shell key not set yet — user can re-run after applying
	}

	sc, ok, err := setup.DetectShellConfig(shell, osName)
	if err != nil {
		return fmt.Errorf("init: detect shell config: %w", err)
	}
	if !ok {
		fmt.Fprintf(cmd.OutOrStdout(), "note: unrecognized shell %q — add the source line manually\n", shell)
		return nil
	}

	has, err := setup.HasSourceLine(sc.RCFile, cfg.initFile)
	if err != nil {
		return fmt.Errorf("init: check RC file: %w", err)
	}
	if has {
		fmt.Fprintf(cmd.OutOrStdout(), "already sourced in %s\n", sc.RCFile)
		return nil
	}

	sourceLine, err := setup.SourceLine(cfg.initFile)
	if err != nil {
		return fmt.Errorf("init: build source line: %w", err)
	}

	if !nonInteractive {
		fmt.Fprintf(cmd.OutOrStdout(), "Add %q to %s? [Y/n]: ", sourceLine, sc.RCFile)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "" && ans != "y" && ans != "yes" {
			fmt.Fprintf(cmd.OutOrStdout(), "skipped — add manually: %s\n", sourceLine)
			return nil
		}
	}

	if err := setup.AppendSourceLine(sc.RCFile, cfg.initFile); err != nil {
		return fmt.Errorf("init: append source line: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "appended source line to %s\n", sc.RCFile)
	return nil
}

func scaffoldDaggerInteractive(reader *bufio.Reader, cmd *cobra.Command, dotfilesPath string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "\nScaffold .dagger convention files? (Enter directory paths, empty to skip)")

	roles := []struct {
		name    string
		content string
	}{
		{"shell scripts directory (source action)", "defaults:\n  actions:\n    - source\n"},
		{"config files directory (link action)", "defaults:\n  actions:\n    - link\n"},
	}

	for _, role := range roles {
		dir, err := promptDefault(cmd.OutOrStdout(), reader, role.name, "", false)
		if err != nil {
			return err
		}
		if dir == "" {
			continue
		}
		dir = filepath.Join(dotfilesPath, filepath.Clean(dir))
		if err := scaffoldDagger(dir, role.content); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/.dagger\n", dir)
	}
	return nil
}

func scaffoldDagger(dir, content string) error {
	path := filepath.Join(dir, ".dagger")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — skip
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// promptDefault prints "msg [default]: " and reads input.
// If input is empty, returns defaultVal. In nonInteractive mode, returns defaultVal immediately.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, defaultVal)
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

func expandTildeStr(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
