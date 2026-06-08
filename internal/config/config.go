// Package config loads and manages the dot-dagger tool configuration file.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// Config key names — single source of truth for Get/Set and command display.
const (
	KeyDotfiles     = "dotfiles"
	KeyBinDir       = "bin_dir"
	KeyGeneratedDir = "generated_dir"
	KeyLinkRoot     = "link_root"
)

// Keys is the ordered list of all valid config keys.
var Keys = []string{KeyDotfiles, KeyBinDir, KeyGeneratedDir, KeyLinkRoot}

// Config holds tool preferences from config.yaml. These are machine-stable settings.
type Config struct {
	Dotfiles     string `yaml:"dotfiles"`
	BinDir       string `yaml:"bin_dir"`
	GeneratedDir string `yaml:"generated_dir"`
	LinkRoot     string `yaml:"link_root"`
}

// DefaultPath returns the default config.yaml path: $XDG_CONFIG_HOME/dot-dagger/config.yaml.
func DefaultPath() (string, error) {
	return ecosystem.DefaultConfigFile()
}

// Load parses config.yaml at path.
// If the file does not exist, returns a zero-value Config without error.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return loadFrom(f)
}

func loadFrom(r io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && err != io.EOF {
		return nil, fmt.Errorf("config: decode: %w", err)
	}
	return &cfg, nil
}

// Save writes cfg to path atomically (temp file + rename). Creates parent dirs.
func Save(path string, cfg *Config) error {
	if err := fileutil.SaveYAML(path, cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// Get returns the value of a named config key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case KeyDotfiles:
		return c.Dotfiles, nil
	case KeyBinDir:
		return c.BinDir, nil
	case KeyGeneratedDir:
		return c.GeneratedDir, nil
	case KeyLinkRoot:
		return c.LinkRoot, nil
	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

// Set updates a named config key.
func (c *Config) Set(key, value string) error {
	switch key {
	case KeyDotfiles:
		c.Dotfiles = value
	case KeyBinDir:
		c.BinDir = value
	case KeyGeneratedDir:
		c.GeneratedDir = value
	case KeyLinkRoot:
		c.LinkRoot = value
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	return nil
}
