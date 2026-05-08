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
const ConfigFile = ".dagger"

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

// DefaultBinDir returns the default path for user-managed binaries: ~/.local/bin/dot-dagger.
// This follows the FHS convention for user-local executables (not an XDG path).
func DefaultBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin", Name), nil
}

// DefaultGeneratedDir returns the default path to the compose generated-files directory:
// $XDG_DATA_HOME/dot-dagger/generated.
func DefaultGeneratedDir() (string, error) {
	base, err := xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "generated"), nil
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

// DefaultLinkRoot returns the default symlink root: $HOME.
func DefaultLinkRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return home, nil
}

// ResolvePath returns the first non-empty value from: cliArg, os.Getenv(envVar),
// envFileVal, then the result of defaultFn. Tilde expansion is not applied here —
// callers are responsible for expanding paths from env.yaml if needed.
func ResolvePath(cliArg, envVar, envFileVal string, defaultFn func() (string, error)) (string, error) {
	if cliArg != "" {
		return cliArg, nil
	}
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	if envFileVal != "" {
		return envFileVal, nil
	}
	return defaultFn()
}
