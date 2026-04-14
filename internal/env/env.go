// Package env resolves the runtime environment map used for predicate evaluation.
// Built-in keys (os, distro, shell) are auto-detected. Custom keys are registered
// via detectors or supplied through an env.yaml file.
package env

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Detector detects the value of a single env key.
type Detector func() (string, error)

// Resolver resolves the full environment map.
// Built-in detectors for "os", "distro", and "shell" are always registered.
// Additional detectors can be added via Register.
type Resolver struct {
	// Detectors maps env key names to detector functions.
	// Public to allow test replacement of built-in detectors.
	Detectors map[string]Detector
}

// NewResolver returns a Resolver pre-loaded with built-in detectors.
func NewResolver() *Resolver {
	return &Resolver{
		Detectors: map[string]Detector{
			"os":     detectOS,
			"distro": detectDistro,
			"shell":  detectShell,
		},
	}
}

// Register adds or replaces a detector for key.
func (r *Resolver) Register(key string, d Detector) {
	r.Detectors[key] = d
}

// Resolve runs all detectors and merges with overrides.
// Overrides have highest precedence and bypass detection for their keys.
// Returns the resolved map and any detector errors encountered.
// Detector errors are non-fatal: the key is omitted from the map, but
// resolution continues. Callers check for *MissingKeysError if specific
// keys are required.
func (r *Resolver) Resolve(overrides map[string]string) (map[string]string, error) {
	env := make(map[string]string)
	var errs []error

	for key, d := range r.Detectors {
		if _, overridden := overrides[key]; overridden {
			continue // skip detection; override wins
		}
		val, err := d()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue
		}
		env[key] = val
	}

	for k, v := range overrides {
		env[k] = v
	}

	return env, errors.Join(errs...)
}

// MissingKeysError is returned when required env keys are absent after resolution.
type MissingKeysError struct {
	Keys []string
}

func (e *MissingKeysError) Error() string {
	return fmt.Sprintf("env: required keys not set: %v", e.Keys)
}

// RequireKeys checks that all keys are present in env.
// Returns *MissingKeysError if any are absent.
func RequireKeys(env map[string]string, keys ...string) error {
	var missing []string
	for _, k := range keys {
		if _, ok := env[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return &MissingKeysError{Keys: missing}
	}
	return nil
}

// EnvFile holds values loaded from an env.yaml file.
type EnvFile struct {
	Env          map[string]string `yaml:"env"`
	DotfilesRepo string            `yaml:"dotfiles_repo"`
}

// LoadEnvFile parses an env.yaml from r.
func LoadEnvFile(r io.Reader) (*EnvFile, error) {
	var f EnvFile
	if err := yaml.NewDecoder(r).Decode(&f); err != nil && err != io.EOF {
		return nil, fmt.Errorf("env: decode env.yaml: %w", err)
	}
	if f.Env == nil {
		f.Env = make(map[string]string)
	}
	return &f, nil
}

// LoadEnvFileFromPath reads env.yaml at path.
// If the file does not exist, returns an empty EnvFile without error.
func LoadEnvFileFromPath(path string) (*EnvFile, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &EnvFile{Env: make(map[string]string)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("env: open %s: %w", path, err)
	}
	defer f.Close()
	return LoadEnvFile(f)
}
