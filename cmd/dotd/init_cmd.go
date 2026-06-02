package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/adopter"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold .dagger convention files in your dotfiles repo",
		Long: `Scaffold .dagger convention files in the configured dotfiles repo.

Prompts for shell scripts, config files, and bin scripts directories.
Creates each directory if absent, writes .dagger if absent (idempotent).

Requires config.yaml — run 'dotd setup' first if you haven't already.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, cfg)
		},
	}
}

func runInit(cmd *cobra.Command, cfg *config) error {
	// cfg.configPath is resolved by PersistentPreRunE.
	if _, err := os.Stat(cfg.configPath); os.IsNotExist(err) {
		return fmt.Errorf("no config found — run 'dotd setup' first")
	}

	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintf(out, "%s — scaffold convention directories\n", ui.Header("dotd init"))
	fmt.Fprintf(out, "Directories are created inside: %s\n", ui.Key(cfg.files))

	written, err := scaffoldDaggerInteractive(out, reader, cfg.files)
	if err != nil {
		return err
	}

	fmt.Fprintln(out)
	for _, path := range written {
		ui.OKf(out, "  wrote %s", path)
	}

	ui.Headerf(out, "Next steps:")
	fmt.Fprintln(out, "  1. Add dotfiles to your repo")
	fmt.Fprintf(out, "  2. %s\n", ui.Key("dotd apply"))
	return nil
}

type conventionRole struct {
	label   string
	desc    string
	defDir  string
	content string
}

var conventionRoles = []conventionRole{
	{
		label:  "Shell scripts",
		desc:   "Files here are auto-sourced by your shell on startup.",
		defDir: adopter.DirShellrc,
		content: "defaults:\n  actions:\n    - source\n",
	},
	{
		label:  "Config files",
		desc:   "Files here are symlinked into ~/.config by default (e.g. config/nvim/init.lua → ~/.config/nvim/init.lua).",
		defDir: adopter.DirConfig,
		content: "link_root: \"~/.config\"\ndefaults:\n  actions:\n    - link\n",
	},
	{
		label:  "Bin scripts",
		desc:   "Executable scripts here are linked to your bin directory.",
		defDir: adopter.DirBin,
		content: "link_root: \"~bin\"\ndefaults:\n  actions:\n    - link\n",
	},
}

// scaffoldDaggerInteractive prompts for each convention dir and scaffolds .dagger files.
// Returns the absolute paths that were written.
func scaffoldDaggerInteractive(out io.Writer, reader *bufio.Reader, dotfilesPath string) ([]string, error) {
	var written []string

	for _, role := range conventionRoles {
		printField(out, role.label, role.desc)

		yes, err := promptYN(out, reader, "Create this directory?")
		if err != nil {
			return written, err
		}
		if !yes {
			ui.Skipf(out, "  skipping")
			continue
		}

		dirName, err := promptDefault(out, reader, fieldPrompt()+" name", role.defDir, false)
		if err != nil {
			return written, err
		}
		if dirName == "" {
			dirName = role.defDir
		}

		dir := filepath.Join(dotfilesPath, filepath.Clean(dirName))
		if err := scaffoldDagger(dir, role.content); err != nil {
			return written, err
		}
		written = append(written, filepath.Join(dir, ecosystem.ConfigFile))
	}

	return written, nil
}

func scaffoldDagger(dir, content string) error {
	path := filepath.Join(dir, ecosystem.ConfigFile)
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — skip
	}
	if err := os.MkdirAll(dir, fileutil.ModeDir); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), fileutil.ModeFile)
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, ui.Skip(defaultVal))
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

// promptYN prints "msg [Y/n]: " and returns true unless user types n/no.
// Empty input and EOF both default to yes.
func promptYN(w io.Writer, reader *bufio.Reader, msg string) (bool, error) {
	fmt.Fprintf(w, "  %s [Y/n]: ", msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		return true, nil // EOF → default yes
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes", nil
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
