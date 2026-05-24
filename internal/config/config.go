// Package config loads and manages the dot-dagger tool configuration file.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"gopkg.in/yaml.v3"
)

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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}

// Get returns the value of a named config key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "dotfiles":
		return c.Dotfiles, nil
	case "bin_dir":
		return c.BinDir, nil
	case "generated_dir":
		return c.GeneratedDir, nil
	case "link_root":
		return c.LinkRoot, nil
	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

// Set updates a named config key.
func (c *Config) Set(key, value string) error {
	switch key {
	case "dotfiles":
		c.Dotfiles = value
	case "bin_dir":
		c.BinDir = value
	case "generated_dir":
		c.GeneratedDir = value
	case "link_root":
		c.LinkRoot = value
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	return nil
}
