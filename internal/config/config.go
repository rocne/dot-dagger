// Package config loads and manages the dot-dagger tool configuration file.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"

	"github.com/rocne/dot-dagger/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// Config key names — single source of truth for Get/Set and command display.
const KeyDotfiles = "dotfiles"

// Keys is the ordered list of all valid config keys.
var Keys = []string{KeyDotfiles}

// Config holds tool preferences from config.yaml. These are machine-stable settings.
type Config struct {
	Dotfiles string `yaml:"dotfiles"`
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

// LoadLenient parses config.yaml like Load but does NOT reject unknown fields;
// instead it returns the names of any it ignored. It is used by the path
// resolution preamble, which runs for every command and must not hard-fail on a
// legacy or partially-corrupt config — otherwise a stale config left over from
// an older schema would block even `dotd teardown`, whose job is to remove it.
//
// Strict validation stays in Load, used by the `config` subcommands where the
// user is directly managing config and should be told about bad fields.
//
// A non-existent file yields a zero-value Config and no unknown fields. Genuine
// YAML syntax errors are still returned.
func LoadLenient(path string) (*Config, []string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("config: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	// First pass into a map to detect unknown keys without failing on them.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("config: decode: %w", err)
	}
	known := make(map[string]bool, len(Keys))
	for _, k := range Keys {
		known[k] = true
	}
	var unknown []string
	for k := range raw {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)

	// Second pass into the struct, ignoring unknown fields.
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("config: decode: %w", err)
	}
	return &cfg, unknown, nil
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
	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

// Set updates a named config key.
func (c *Config) Set(key, value string) error {
	switch key {
	case KeyDotfiles:
		c.Dotfiles = value
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	return nil
}
