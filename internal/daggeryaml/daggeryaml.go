// Package daggeryaml loads and validates .dot-dagger.yaml per-directory config files.
// These files are ecosystem config — sections are owned by individual tools but the
// file belongs to the suite as a whole.
package daggeryaml

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the parsed contents of a .dot-dagger.yaml file.
// Each section is owned by the corresponding tool.
// Missing sections are zero-valued, not errors.
type Config struct {
	Dote DoteSection `yaml:"dote"`
	Dotd DotdSection `yaml:"dotd"`
	Dotl DotlSection `yaml:"dotl"`
}

// DoteSection holds dote-owned config: env overrides for this directory subtree.
type DoteSection struct {
}

// DotdSection holds dotd-owned config: directory-level when, cascading defaults,
// and per-file metadata for files that cannot carry annotations.
type DotdSection struct {
	When     string       `yaml:"when"`
	Defaults DotdDefaults `yaml:"defaults"`
	Files    []FileEntry  `yaml:"files"`
}

// DotdDefaults holds values that cascade to all files within the directory.
type DotdDefaults struct {
	When string `yaml:"when"`
}

// FileEntry provides annotation metadata for a file that cannot carry annotations
// (e.g. JSON, binary). Path is the true filename on disk.
type FileEntry struct {
	Path         string `yaml:"path"`
	When         string `yaml:"when"`
	After        string `yaml:"after"`
	Name         string `yaml:"name"`
	Symlink      string `yaml:"symlink"`
	RetainPrefix bool   `yaml:"retain_prefix"`
	Disable      bool   `yaml:"disable"`   // equivalent to @disable
	NoSource     bool   `yaml:"no_source"` // equivalent to @no-source
	Source       bool   `yaml:"source"`    // equivalent to @source
}

// DotlSection holds dotl-owned config: symlink root override.
type DotlSection struct {
	LinkRoot string `yaml:"link_root"`
}

// Load parses a .dot-dagger.yaml from r.
func Load(r io.Reader) (*Config, error) {
	var d Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil && err != io.EOF {
		return nil, fmt.Errorf("daggeryaml: decode: %w", err)
	}
	return &d, nil
}

// LoadFile reads a .dot-dagger.yaml at path.
// If the file does not exist, returns a zero-value Config without error.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daggeryaml: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}
