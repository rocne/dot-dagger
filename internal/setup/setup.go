// Package setup scaffolds a dotfiles repo structure and config files for first-time use.
// All operations are idempotent: existing files and directories are skipped without error.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options configures the scaffold operation.
type Options struct {
	DotfilesDir  string
	EnvFilePath  string
	InitFilePath string
	// Detected values shown as comments in the generated env.yaml.
	DetectedOS     string
	DetectedDistro string
	DetectedShell  string
}

// ActionKind describes what kind of thing was created.
type ActionKind string

const (
	KindDir  ActionKind = "dir"
	KindFile ActionKind = "file"
)

// ActionState describes the outcome of a setup action.
type ActionState string

const (
	StateCreated ActionState = "created"
	StateSkipped ActionState = "skipped"
)

// Action is one step taken (or skipped) during scaffold.
type Action struct {
	Kind  ActionKind
	Path  string
	State ActionState
}

// Result holds the actions taken during scaffold.
type Result struct {
	Actions []Action
}

// Scaffold creates the dotfiles repo structure and config files.
// Idempotent: existing files and directories are skipped without error.
func Scaffold(opts Options) (*Result, error) {
	var res Result

	// Repo directories owned by suite tools.
	for _, sub := range []string{"scripts", "conf", "bin"} {
		act, err := ensureDir(filepath.Join(opts.DotfilesDir, sub))
		if err != nil {
			return nil, err
		}
		res.Actions = append(res.Actions, act)
	}

	// Config files — only written if absent.
	files := []struct {
		path    string
		content string
	}{
		{opts.EnvFilePath, envYAML(opts)},
		{filepath.Join(opts.DotfilesDir, ".dotr.yaml"), dotrYAML()},
		{filepath.Join(opts.DotfilesDir, "packages.yaml"), packagesYAML()},
	}
	for _, f := range files {
		act, err := ensureFile(f.path, f.content)
		if err != nil {
			return nil, err
		}
		res.Actions = append(res.Actions, act)
	}

	return &res, nil
}

func ensureDir(path string) (Action, error) {
	if _, err := os.Stat(path); err == nil {
		return Action{Kind: KindDir, Path: path, State: StateSkipped}, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Action{}, fmt.Errorf("setup: create dir %s: %w", path, err)
	}
	return Action{Kind: KindDir, Path: path, State: StateCreated}, nil
}

func ensureFile(path, content string) (Action, error) {
	if _, err := os.Stat(path); err == nil {
		return Action{Kind: KindFile, Path: path, State: StateSkipped}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Action{}, fmt.Errorf("setup: mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Action{}, fmt.Errorf("setup: write %s: %w", path, err)
	}
	return Action{Kind: KindFile, Path: path, State: StateCreated}, nil
}

// --- file content generators ---

func envYAML(opts Options) string {
	var sb strings.Builder
	sb.WriteString("# dotr environment — keys used in @when predicates.\n")
	sb.WriteString("# Built-in keys are auto-detected at runtime. Uncomment to override.\n")
	if opts.DetectedOS != "" || opts.DetectedDistro != "" || opts.DetectedShell != "" {
		sb.WriteString("\n")
	}
	if opts.DetectedOS != "" {
		fmt.Fprintf(&sb, "# os: %s\n", opts.DetectedOS)
	}
	if opts.DetectedDistro != "" {
		fmt.Fprintf(&sb, "# distro: %s\n", opts.DetectedDistro)
	}
	if opts.DetectedShell != "" {
		fmt.Fprintf(&sb, "# shell: %s\n", opts.DetectedShell)
	}
	return sb.String()
}

func dotrYAML() string {
	return `# Per-directory config for non-annotatable files.
# Each tool reads only its own section.

dotd:
  # when: "os == linux"
  # defaults:
  #   when: "os == linux"

dotl:
  # link_root: ~/.config/nvim

dote:
  # env overrides for this directory subtree
`
}

func packagesYAML() string {
	return `# Package registry — maps logical package names to package manager entries.
# Used by @require (hard gate) and @request (soft ask) annotations.
#
# Example:
# packages:
#   neovim:
#     managers:
#       brew: { install: "brew install neovim" }
#       apt:  { install: "apt-get install -y neovim" }
`
}
