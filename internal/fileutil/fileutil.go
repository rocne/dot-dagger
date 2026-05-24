// Package fileutil provides atomic file write helpers.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveYAML encodes v as YAML and writes it to path atomically (temp file + rename).
// Creates parent directories as needed.
func SaveYAML(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("fileutil: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("fileutil: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: rename: %w", err)
	}
	return nil
}
