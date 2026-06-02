package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/adopter"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/setup"
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

	if err := maybeAddSourceLine(out, reader, cfg); err != nil {
		return err
	}

	ui.Headerf(out, "Next steps:")
	fmt.Fprintln(out, "  1. Add dotfiles to your repo")
	fmt.Fprintf(out, "  2. %s\n", ui.Key("dotd apply"))
	return nil
}

// maybeAddSourceLine checks if the shell RC file already sources the dotd init
// file and, if not, prompts the user to add it. Shell and OS are resolved from
// the canonical env map (never queried directly from the OS here).
func maybeAddSourceLine(out io.Writer, reader *bufio.Reader, cfg *config) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		// env.yaml not yet present (edge case); skip silently.
		return nil
	}
	shell := resolved["shell"]
	if shell == "" {
		return nil
	}
	sc, ok, err := setup.DetectShellConfig(shell, resolved["os"], cfg.linkRoot)
	if err != nil {
		return fmt.Errorf("init: detect shell config: %w", err)
	}
	if !ok {
		return nil // unrecognised shell — skip
	}

	has, err := setup.HasSourceLine(sc.RCFile, cfg.initFile)
	if err != nil {
		return fmt.Errorf("init: check RC file: %w", err)
	}
	if has {
		ui.OKf(out, "  source line already present in %s", sc.RCFile)
		return nil
	}

	yes, err := promptYN(out, reader, fmt.Sprintf("Add dotd source line to %s?", sc.RCFile))
	if err != nil {
		return err
	}
	if !yes {
		ui.Skipf(out, "  skipping source line — add manually: source %q", cfg.initFile)
		return nil
	}

	if err := setup.AppendSourceLine(sc.RCFile, cfg.initFile, cfg.linkRoot); err != nil {
		return fmt.Errorf("init: append source line: %w", err)
	}
	ui.OKf(out, "  added source line to %s", sc.RCFile)
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

