// Package dotryaml loads and validates .dotr.yaml per-directory config files.
package dotryaml

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// DotR holds the parsed contents of a .dotr.yaml file.
// Each section is owned by the corresponding tool.
// Missing sections are zero-valued, not errors.
type DotR struct {
	Dote DoteSection `yaml:"dote"`
	Dotd DotdSection `yaml:"dotd"`
	Dotl DotlSection `yaml:"dotl"`
}

// DoteSection holds dote-owned config: env overrides and package manager priority.
type DoteSection struct {
	PackageManagers PackageManagersConfig `yaml:"package_managers"`
}

// PackageManagersConfig declares the ordered preference for package managers.
type PackageManagersConfig struct {
	Priority []string `yaml:"priority"`
}

// DotdSection holds dotd-owned config: directory-level when and cascading defaults.
type DotdSection struct {
	When     string       `yaml:"when"`
	Defaults DotdDefaults `yaml:"defaults"`
}

// DotdDefaults holds values that cascade to all files within the directory.
type DotdDefaults struct {
	When string `yaml:"when"`
}

// DotlSection holds dotl-owned config: symlink root override.
type DotlSection struct {
	LinkRoot string `yaml:"link_root"`
}

// Load parses a .dotr.yaml from r.
func Load(r io.Reader) (*DotR, error) {
	var d DotR
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil && err != io.EOF {
		return nil, fmt.Errorf("dotryaml: decode: %w", err)
	}
	return &d, nil
}

// LoadFile reads a .dotr.yaml at path.
// If the file does not exist, returns a zero-value DotR without error.
func LoadFile(path string) (*DotR, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &DotR{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dotryaml: open %s: %w", path, err)
	}
	defer f.Close()
	return Load(f)
}
