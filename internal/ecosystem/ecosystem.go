// Package ecosystem holds the canonical name, tool names, and default paths for the dot-dagger suite.
// All tools import from here — there is exactly one place to change names or paths.
package ecosystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// Name is the canonical ecosystem name. Used in config paths, data paths, and user-facing output.
const Name = "dot-dagger"

// ConfigFile is the per-directory config filename placed inside dotfiles repos.
const ConfigFile = "." + Name + ".yaml" // .dot-dagger.yaml

// Tool names — the CLI binary name for each suite member.
const (
	ToolR = "dotr" // orchestrator
	ToolD = "dotd" // DAG / init.sh generation
	ToolE = "dote" // environment resolution
	ToolL = "dotl" // symlink management
	ToolP = "dotp" // package management
)

// DefaultEnvFile returns the default path to env.yaml: ~/.config/dot-dagger/env.yaml.
func DefaultEnvFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", Name, "env.yaml"), nil
}

// DefaultInitFile returns the default path to init.sh: ~/.config/dot-dagger/init.sh.
func DefaultInitFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", Name, "init.sh"), nil
}

// DefaultDotfiles returns the default path to the dotfiles repo.
// Reads $DOTFILES env var; falls back to the current working directory.
func DefaultDotfiles() string {
	if d, ok := os.LookupEnv("DOTFILES"); ok {
		return d
	}
	dir, _ := os.Getwd()
	return dir
}
