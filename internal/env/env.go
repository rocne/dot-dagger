// Package env resolves the runtime environment map used for predicate evaluation.
// env.yaml is a flat YAML map[string]string; values may contain $(…) shell expressions.
package env

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// dotdPrefix is the shell-variable prefix used to override env values.
// DOTD_CONTEXT=work → context=work in the resolved env map.
const dotdPrefix = "DOTD_"

// Load parses env.yaml at path as a flat map[string]string.
// Values are returned raw — call Expand to evaluate $(…) expressions.
// If the file does not exist, returns an empty map without error.
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("env: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return load(f)
}

func load(r io.Reader) (map[string]string, error) {
	var raw map[string]string
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil && err != io.EOF {
		return nil, fmt.Errorf("env: decode: %w", err)
	}
	if raw == nil {
		raw = map[string]string{}
	}
	return raw, nil
}

// Expand evaluates $(…) shell expressions in raw values using sh -c.
// Failed or empty commands produce an empty string — the key remains in the map.
// Non-expression values are passed through unchanged.
func Expand(raw map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		cmd, ok := shellExpr(v)
		if !ok {
			out[k] = v
			continue
		}
		result, err := runShell(cmd)
		if err != nil {
			out[k] = ""
			continue
		}
		out[k] = result
	}
	return out, nil
}

// shellExpr returns the command inside $(…) and true if v matches that pattern.
func shellExpr(v string) (string, bool) {
	if !strings.HasPrefix(v, "$(") || !strings.HasSuffix(v, ")") {
		return "", false
	}
	return v[2 : len(v)-1], true
}

// runShell runs cmd via sh -c and returns trimmed stdout.
func runShell(cmd string) (string, error) {
	var buf bytes.Buffer
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = &buf
	if err := c.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// Resolve merges env layers in precedence order (highest → lowest):
// cliFlags > shellVars > expanded.
func Resolve(cliFlags, shellVars, expanded map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range expanded {
		out[k] = v
	}
	for k, v := range shellVars {
		out[k] = v
	}
	for k, v := range cliFlags {
		out[k] = v
	}
	return out
}

// parseFlags parses "key=val,key2=val2" into a map.
// Empty string returns an empty map. Returns error if any entry lacks =.
func parseFlags(s string) (map[string]string, error) {
	if s == "" {
		return map[string]string{}, nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			return nil, fmt.Errorf("env: invalid key=value %q", pair)
		}
		out[pair[:idx]] = pair[idx+1:]
	}
	return out, nil
}

// ShellVars extracts DOTD_* vars from environ, lowercasing the suffix as the key.
// DOTD_CONTEXT=work → context=work. Entries with empty suffix are ignored.
func ShellVars(environ []string) map[string]string {
	out := make(map[string]string)
	for _, e := range environ {
		if !strings.HasPrefix(e, dotdPrefix) {
			continue
		}
		rest := e[len(dotdPrefix):]
		idx := strings.IndexByte(rest, '=')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(rest[:idx])
		val := rest[idx+1:]
		out[key] = val
	}
	return out
}

// DefaultPath returns the default env.yaml path: $XDG_CONFIG_HOME/dot-dagger/env.yaml.
func DefaultPath() (string, error) {
	return ecosystem.DefaultEnvFile()
}

// Save writes raw to path atomically (temp file + rename). Creates parent dirs.
func Save(path string, raw map[string]string) error {
	if err := fileutil.SaveYAML(path, raw); err != nil {
		return fmt.Errorf("env: %w", err)
	}
	return nil
}
