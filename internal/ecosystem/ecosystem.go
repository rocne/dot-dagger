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

// LegacyConfigFile is the old per-directory config filename, kept for defensive skipping during walks.
const LegacyConfigFile = ".dotd.yaml"

// PackagesFileName is the canonical filename for the packages registry inside a dotfiles repo.
const PackagesFileName = "packages.yaml"

// EnvFileName is the canonical filename for the per-machine env configuration.
const EnvFileName = "env.yaml"

// ToolD is the CLI binary name.
const ToolD = "dotd"

// XdgConfigHome returns $XDG_CONFIG_HOME if set to an absolute path, else ~/.config.
// Use this as the canonical XDG config home — do not call os.Getenv("XDG_CONFIG_HOME") directly.
func XdgConfigHome() (string, error) { return xdgConfigHome() }

// userHome wraps os.UserHomeDir with a package-uniform error message.
func userHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ecosystem: cannot determine home directory: %w", err)
	}
	return home, nil
}

// xdgConfigHome is the unexported implementation shared by all Default* functions in this package.
func xdgConfigHome() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// xdgDataHome returns $XDG_DATA_HOME if set to an absolute path, else ~/.local/share.
func xdgDataHome() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// DefaultConfigFile returns the default path to config.yaml: $XDG_CONFIG_HOME/dot-dagger/config.yaml.
func DefaultConfigFile() (string, error) {
	base, err := xdgConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "config.yaml"), nil
}

// DefaultEnvFile returns the default path to env.yaml: $XDG_CONFIG_HOME/dot-dagger/env.yaml.
func DefaultEnvFile() (string, error) {
	base, err := xdgConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "env.yaml"), nil
}

// InitFile returns the path to init.sh: $XDG_DATA_HOME/dot-dagger/init.sh.
// init.sh is generated output, not user-edited config — it belongs in XDG_DATA_HOME.
func InitFile() (string, error) {
	base, err := xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "init.sh"), nil
}

// GeneratedDir returns the path to the compose generated-files directory:
// $XDG_DATA_HOME/dot-dagger/generated.
func GeneratedDir() (string, error) {
	base, err := xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name, "generated"), nil
}

// Home returns the user's home directory ($HOME on linux/darwin) — the single
// canonical accessor for "~". Not a configurable knob: $HOME is authoritative
// (universal convention, like $EDITOR).
func Home() (string, error) {
	return userHome()
}

// xdgBinHome returns $XDG_BIN_HOME if set to an absolute path, else ~/.local/bin.
// $XDG_BIN_HOME is not in the XDG base spec but is the de-facto convention for
// user binaries; honoring it lets users relocate the bin root the standard way.
func xdgBinHome() (string, error) {
	if d := os.Getenv("XDG_BIN_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// BinDir returns the dot-dagger-namespaced bin route: <xdgBinHome>/dot-dagger.
// Namespacing is free because PATH is a search list; init.sh adds it to PATH.
func BinDir() (string, error) {
	base, err := xdgBinHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name), nil
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

// GeneratedFileHeader is the comment header for every file dotd generates
// (init.sh, bundles, install scripts) — one identity, one place to change it.
func GeneratedFileHeader() string {
	return "# Generated by " + Name + " — do not edit by hand."
}
