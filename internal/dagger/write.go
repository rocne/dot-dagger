package dagger

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// linkAction returns the string action form "link(<dest>)" — the same shape
// normalizeActionAnnotations produces and the files:-dict walk path parses.
func linkAction(dest string) string {
	return "link(" + dest + ")"
}

// FileLinkSnippet returns the exact YAML block that records a files: entry
// giving name a single explicit link action to dest. Used both as the fresh
// content / textual append written by SetFileLink and as the copy-paste
// snippet shown to the user when SetFileLink cannot mutate safely.
func FileLinkSnippet(name, dest string) string {
	return "files:\n" +
		"  " + yamlScalar(name) + ":\n" +
		"    actions:\n" +
		"      - " + yamlScalar(linkAction(dest)) + "\n"
}

// yamlScalar encodes s as a single-line YAML scalar, quoting only when the
// plain form would not round-trip.
func yamlScalar(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return s // unreachable: strings always marshal
	}
	return strings.TrimSuffix(string(b), "\n")
}

// SetFileLink records files.<name>.actions: ["link(<dest>)"] in the .dagger
// file at path without disturbing any existing content or comments. Mutation
// tiers, most- to least-common:
//
//  1. path does not exist (or is empty) — write a fresh files: block;
//  2. .dagger exists without a files: key — textual append, preserving the
//     existing bytes verbatim;
//  3. files: exists — yaml.Node-level surgery appending one entry.
//
// The mutated bytes are re-parsed and checked for the expected entry before
// anything is written; on any doubt SetFileLink returns an error and writes
// nothing, so the caller can fall back to instructing the user with
// FileLinkSnippet.
func SetFileLink(path, name, dest string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("dagger: read %s: %w", path, err)
	}

	newData, err := insertFileLink(data, name, dest)
	if err != nil {
		return err
	}

	// Verify before writing: the mutated bytes must load cleanly (Load's own
	// strictness included — KnownFields, explicit-null rejection) and contain
	// exactly the entry being recorded.
	cfg, err := Load(bytes.NewReader(newData))
	if err != nil {
		return fmt.Errorf("dagger: mutated %s would not parse: %w", path, err)
	}
	entry, ok := cfg.Files[name]
	if !ok || len(entry.Actions) != 1 || entry.Actions[0] != linkAction(dest) {
		return fmt.Errorf("dagger: mutated %s does not contain the expected files entry for %q", path, name)
	}

	return fileutil.WriteAtomic(path, newData, fileutil.ModeFile)
}

// insertFileLink returns data with a files.<name> link entry added, choosing
// the least invasive mutation tier (see SetFileLink).
func insertFileLink(data []byte, name, dest string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("dagger: parse existing content: %w", err)
	}

	// Missing, empty, or comments-only file: appending keeps any comment
	// bytes verbatim (an absent file appends onto nothing → fresh content).
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return appendSnippet(data, name, dest), nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("dagger: top level is not a mapping")
	}

	filesVal := findMapValue(root, "files")
	if filesVal == nil && root.Style&yaml.FlowStyle == 0 {
		// Block-style mapping without files: — textual append preserves the
		// user's file byte-for-byte, comments and formatting included.
		return appendSnippet(data, name, dest), nil
	}

	// files: exists (or the root mapping is flow-style, where textual append
	// would be invalid YAML): yaml.Node surgery. Node round-trips preserve
	// comments; the verify pass in SetFileLink backstops any encoder quirk.
	entryKey := strNode(name)
	entryVal := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		strNode("actions"),
		{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{
			strNode(linkAction(dest)),
		}},
	}}

	// The appended entry nodes are block-style; a flow-style container around
	// them would render invalid YAML. Normalize only the containers we touch —
	// nested nodes keep their own styles.
	root.Style &^= yaml.FlowStyle
	if filesVal != nil {
		filesVal.Style &^= yaml.FlowStyle
	}

	switch {
	case filesVal == nil:
		root.Content = append(root.Content,
			strNode("files"),
			&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{entryKey, entryVal}})
	case filesVal.Tag == "!!null":
		// files: with no value — replace the null with a mapping.
		*filesVal = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{entryKey, entryVal}}
	case filesVal.Kind == yaml.MappingNode:
		if findMapValue(filesVal, name) != nil {
			return nil, fmt.Errorf("dagger: a files entry for %q already exists", name)
		}
		filesVal.Content = append(filesVal.Content, entryKey, entryVal)
	default:
		return nil, fmt.Errorf("dagger: files: is not a mapping")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("dagger: re-encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("dagger: re-encode close: %w", err)
	}
	return buf.Bytes(), nil
}

// appendSnippet appends the files: block for name/dest to data, inserting a
// newline first if data does not already end with one.
func appendSnippet(data []byte, name, dest string) []byte {
	out := append([]byte(nil), data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, FileLinkSnippet(name, dest)...)
}

// findMapValue returns the value node for key in mapping m, or nil.
func findMapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}
