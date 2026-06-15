// Package fileutil provides atomic file write helpers.
package fileutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModeDir is the standard permission for created directories (rwxr-xr-x).
const ModeDir os.FileMode = 0o755

// ModeFile is the standard permission for written files (rw-r--r--).
const ModeFile os.FileMode = 0o644

// POSIXShebang is the standard POSIX shell shebang line.
const POSIXShebang = "#!/bin/sh"

// Exists reports whether path exists. Any stat error (including ENOENT and
// permission denied) is treated as non-existence.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExpandHome expands "~" → home and "~/x" → home/x. Any other input is
// returned unchanged. Pass "" for home to short-circuit expansion.
func ExpandHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) >= 2 && path[0] == '~' && path[1] == '/' {
		return filepath.Join(home, path[2:])
	}
	return path
}

// SaveYAML encodes v as YAML and writes it to path atomically via WriteAtomic.
// Creates parent directories as needed.
func SaveYAML(path string, v any) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("fileutil: encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("fileutil: encode close: %w", err)
	}
	return WriteAtomic(path, buf.Bytes(), ModeFile)
}

// WriteAtomic writes data to path atomically (temp file in the same dir +
// rename) with the given mode. Creates parent directories. This is the single
// owner of atomic-write mechanics — yaml saves, init.sh generation,
// annotation rewrites, and compose-generated files all route through it.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), ModeDir); err != nil {
		return fmt.Errorf("fileutil: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".*.tmp")
	if err != nil {
		return fmt.Errorf("fileutil: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fileutil: rename: %w", err)
	}
	return nil
}

// ShellQuote returns s safely usable as a single shell word: returned
// unchanged when it contains no shell-special characters, otherwise
// single-quoted with embedded quotes escaped via the '"'"' idiom.
func ShellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\"'\\$`#&|;<>()*?[]~{}") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
