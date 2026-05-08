package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/env"
	"github.com/spf13/cobra"
)

func newInitCmd(_ *config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard: create config.yaml and env.yaml",
		Long: `Walk through creating or updating the tool configuration.

Writes ~/.config/dot-dagger/config.yaml and ~/.config/dot-dagger/env.yaml.
Optionally scaffolds .dagger convention files in your dotfiles repo.

Run without flags for interactive mode. Use --yes to accept all defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: accept all defaults")
	return cmd
}

func runInit(cmd *cobra.Command, nonInteractive bool) error {
	reader := bufio.NewReader(os.Stdin)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Gather tool config values.
	dotfilesDefault := filepath.Join(home, "dotfiles")
	if d, ok := os.LookupEnv("DOTFILES"); ok {
		dotfilesDefault = d
	}

	dotfilesPath, err := promptDefault(reader, "Dotfiles repo path", dotfilesDefault, nonInteractive)
	if err != nil {
		return err
	}
	dotfilesPath = expandTildeStr(dotfilesPath, home)

	binDirDefault := filepath.Join(home, ".local", "bin", "dot-dagger")
	binDir, err := promptDefault(reader, "Bin directory", binDirDefault, nonInteractive)
	if err != nil {
		return err
	}
	binDir = expandTildeStr(binDir, home)

	generatedDirDefault := filepath.Join(home, ".local", "share", "dot-dagger", "generated")
	generatedDir, err := promptDefault(reader, "Generated files directory", generatedDirDefault, nonInteractive)
	if err != nil {
		return err
	}
	generatedDir = expandTildeStr(generatedDir, home)

	linkRootDefault := home
	linkRoot, err := promptDefault(reader, "Default symlink root", linkRootDefault, nonInteractive)
	if err != nil {
		return err
	}
	linkRoot = expandTildeStr(linkRoot, home)

	// Write config.yaml.
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	cfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		LinkRoot:     linkRoot,
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", filepath.Dir(configPath), err)
	}
	if err := dotcfg.Save(configPath, cfg); err != nil {
		return fmt.Errorf("init: save config.yaml: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)

	// Write env.yaml if it doesn't exist.
	envPath, err := env.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
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

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Add dotfiles to %s\n", dotfilesPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: %s apply\n", ecosystem.ToolD)
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
		dir, err := promptDefault(reader, role.name, "", false)
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
func promptDefault(reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", msg, defaultVal)
	} else {
		fmt.Printf("%s: ", msg)
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
