// Package dagger loads and parses .dagger per-directory config files.
package dagger

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// BasicNode is the base metadata that can appear on any node.
// Every field corresponds to an annotation supported in file headers.
type BasicNode struct {
	When     string   `yaml:"when"`
	LinkRoot string   `yaml:"link_root"`
	Actions  []string `yaml:"actions"`
	After    []string `yaml:"after"`
	Require  []string `yaml:"require"`
	Request  []string `yaml:"request"`
	Disable  bool     `yaml:"disable"`
}

// NamedNode extends BasicNode with an optional logical name override.
// Used for entries in the files: dict.
type NamedNode struct {
	BasicNode `yaml:",inline"`
	Name      string `yaml:"name"`
}

// CompositionConfig controls whether this directory is a compose target.
type CompositionConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ConventionConfig holds the names of the three convention directories.
// Zero-value fields mean "use the default" — callers apply defaults when fields are empty.
type ConventionConfig struct {
	Shellrc string `yaml:"shellrc"`
	Bin     string `yaml:"bin"`
	Config  string `yaml:"config"`
}

// ComposableNode is the top-level structure of a .dagger file.
// It represents a directory node with all possible fields.
type ComposableNode struct {
	NamedNode   `yaml:",inline"`
	Defaults    BasicNode            `yaml:"defaults"`
	Files       map[string]NamedNode `yaml:"files"`
	Composition CompositionConfig    `yaml:"composition"`
	Compose     bool                 `yaml:"compose"` // alias for composition.enabled
	Conventions ConventionConfig     `yaml:"conventions"`
}

// IsCompose reports whether this directory is a compose target.
func (c *ComposableNode) IsCompose() bool {
	return c.Composition.Enabled || c.Compose
}

// Load parses a .dagger file from r.
// An empty or missing file is valid and returns a zero-value ComposableNode.
func Load(r io.Reader) (*ComposableNode, error) {
	var node ComposableNode
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&node); err != nil && err != io.EOF {
		return nil, fmt.Errorf("dagger: decode: %w", err)
	}
	return &node, nil
}

// LoadFile reads a .dagger file at path.
// If the file does not exist, returns a zero-value ComposableNode without error.
func LoadFile(path string) (*ComposableNode, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &ComposableNode{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dagger: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}
