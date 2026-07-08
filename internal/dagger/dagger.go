// Package dagger loads and parses .dagger per-directory config files.
package dagger

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// scalarString is a string field that must decode from a plain YAML scalar.
// It closes a footgun: an unquoted "~" (or a key with no value at all) is
// the YAML null literal, not the string "~". Under a plain `string` field,
// yaml.v3 silently decodes null to "" — indistinguishable from the key being
// absent — so a value like `link_root: ~` copied verbatim from prose docs
// looks correct, parses without error, and quietly does nothing.
//
// UnmarshalYAML rejects a non-scalar value (a map or sequence) assigned to a
// string field. It cannot reject an explicit null itself: yaml.v3's decoder
// never calls a field's Unmarshaler for a null-tagged node — it short-circuits
// straight to the zero value before checking any interface (see
// decoder.prepare in gopkg.in/yaml.v3, which returns early whenever
// n.ShortTag() == "!!null"). That's why explicit-null detection for
// when/link_root/name lives in rejectExplicitNull below, which walks the raw
// node tree — the only place "null" and "absent" are still distinguishable.
type scalarString string

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *scalarString) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a plain string, got %s", value.Tag)
	}
	*s = scalarString(value.Value)
	return nil
}

// nullCheckedKeys are the yaml keys whose value must not be an explicit null
// (they decode into scalarString fields, where null and "absent" would
// otherwise be indistinguishable). "when"/"link_root"/"name" don't appear
// anywhere else in the .dagger schema, so a plain key-name match is safe
// without tracking which mapping context they're nested in (top-level,
// defaults:, or a files: entry).
var nullCheckedKeys = map[string]bool{
	"when":      true,
	"link_root": true,
	"name":      true,
}

// rejectExplicitNull walks a parsed YAML document looking for any
// nullCheckedKeys key whose value is an explicit null (bare ~ or empty
// value). Quoting the anchor, e.g. `link_root: "~"`, is required and works
// correctly — this only rejects the unquoted footgun.
func rejectExplicitNull(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if nullCheckedKeys[key.Value] && val.Tag == "!!null" {
				return fmt.Errorf("line %d: %s is null — an unquoted ~ (or a key with no value) is YAML null, not the string \"~\"; quote it, e.g. write %s: \"~\"", key.Line, key.Value, key.Value)
			}
		}
	}
	for _, c := range n.Content {
		if err := rejectExplicitNull(c); err != nil {
			return err
		}
	}
	return nil
}

// BasicNode is the base metadata that can appear on any node.
// Every field corresponds to an annotation supported in file headers.
type BasicNode struct {
	When     scalarString `yaml:"when"`
	LinkRoot scalarString `yaml:"link_root"`
	Actions  []string     `yaml:"actions"`
	After    []string     `yaml:"after"`
	Require  []string     `yaml:"require"`
	Request  []string     `yaml:"request"`
	Disable  bool         `yaml:"disable"`
}

// NamedNode extends BasicNode with an optional logical name override.
// Used for entries in the files: dict.
type NamedNode struct {
	BasicNode `yaml:",inline"`
	Name      scalarString `yaml:"name"`
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
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dagger: read: %w", err)
	}

	// Raw-node pass: catch explicit null on when/link_root/name before the
	// strict struct decode below, which cannot see it (scalarString's
	// UnmarshalYAML is never invoked for a null node — see its doc comment).
	var raw yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&raw); err != nil && err != io.EOF {
		return nil, fmt.Errorf("dagger: decode: %w", err)
	}
	if err := rejectExplicitNull(&raw); err != nil {
		return nil, fmt.Errorf("dagger: %w", err)
	}

	var node ComposableNode
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&node); err != nil && err != io.EOF {
		return nil, fmt.Errorf("dagger: decode: %w", err)
	}
	// A .dagger file is a single config document. Reject trailing YAML
	// documents (extra `---` separators) rather than silently dropping them —
	// consistent with the KnownFields(true) strictness above.
	if err := dec.Decode(new(ComposableNode)); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("dagger: decode: %w", err)
		}
		return nil, fmt.Errorf("dagger: multiple YAML documents in .dagger file (only one allowed)")
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
		return nil, fmt.Errorf("dagger: %w", err) // *PathError already names the path
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}
