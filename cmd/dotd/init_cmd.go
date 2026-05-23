package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold .dagger convention files in your dotfiles repo",
		Long: `Scaffold .dagger convention files in the configured dotfiles repo.

Prompts for shell scripts and config file directories.
Creates each directory if absent, writes .dagger if absent (idempotent).

Requires config.yaml — run 'dotd setup' first if you haven't already.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, cfg)
		},
	}
}

func runInit(cmd *cobra.Command, cfg *config) error {
	// Precondition: config.yaml must exist. DefaultPath() called directly here —
	// bootstrap check that runs before any config is loaded (legitimate exception).
	configPath, err := dotcfg.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("no config found — run 'dotd setup' first")
	}

	reader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintln(cmd.OutOrStdout(), "Scaffold .dagger convention files — enter directory paths, empty to skip.")
	fmt.Fprintln(cmd.OutOrStdout())

	if err := scaffoldDaggerInteractive(reader, cmd, cfg.files); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Add dotfiles to your repo")
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: %s apply\n", ecosystem.ToolD)
	return nil
}

func scaffoldDaggerInteractive(reader *bufio.Reader, cmd *cobra.Command, dotfilesPath string) error {
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
// Returns defaultVal if input is empty.
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
