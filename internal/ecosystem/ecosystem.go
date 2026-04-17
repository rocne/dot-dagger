// Package ecosystem holds the canonical name, tool name, and default paths for dotd.
// All packages import from here — there is exactly one place to change names or paths.
package ecosystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// Name is the canonical ecosystem name. Used in config paths, data paths, and user-facing output.
const Name = "dot-dagger"

// ConfigFile is the per-directory config filename placed inside dotfiles repos.
const ConfigFile = "." + ToolD + ".yaml" // .dotd.yaml

// ToolD is the CLI binary name.
const ToolD = "dotd"

// xdgConfigHome returns $XDG_CONFIG_HOME if set to an absolute path, else ~/.config.
func xdgConfigHome() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

// xdgDataHome returns $XDG_DATA_HOME if set to an absolute path, else ~/.local/share.
func xdgDataHome() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// DefaultEnvFile returns the default path to env.yaml: $XDG_CONFIG_HOME/dot-dagger/env.yaml.
func DefaultEnvFile() (string, error) {
	base, err := xdgConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "env.yaml"), nil
}

// DefaultInitFile returns the default path to init.sh: $XDG_DATA_HOME/dot-dagger/init.sh.
// init.sh is generated output, not user-edited config — it belongs in XDG_DATA_HOME.
func DefaultInitFile() (string, error) {
	base, err := xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "init.sh"), nil
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
