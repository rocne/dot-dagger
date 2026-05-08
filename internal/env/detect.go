package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvFileConfig holds the path-override keys that env.yaml may define.
// These are loaded by LoadEnvFileFromPath and used to fill in CLI defaults.
type EnvFileConfig struct {
	DotfilesRepo string
	LinkRoot     string
	BinDir       string
	GeneratedDir string
	InitFile     string
}

// LoadEnvFileFromPath reads the path-override keys from the env.yaml at path.
// If the file does not exist, returns a zero-value struct without error.
func LoadEnvFileFromPath(path string) (*EnvFileConfig, error) {
	raw, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &EnvFileConfig{
		DotfilesRepo: raw["dotfiles_repo"],
		LinkRoot:     raw["link_root"],
		BinDir:       raw["bin_dir"],
		GeneratedDir: raw["generated_dir"],
		InitFile:     raw["init_file"],
	}, nil
}

// ResolveWithOverrides loads env.yaml, expands $(…) shell expressions, merges
// DOTD_* environment variables, then applies cliOverrides (key=val strings) on top.
func ResolveWithOverrides(envFilePath string, cliOverrides []string) (map[string]string, error) {
	raw, err := Load(envFilePath)
	if err != nil {
		return nil, err
	}
	expanded, err := Expand(raw)
	if err != nil {
		return nil, err
	}
	flags := make(map[string]string, len(cliOverrides))
	for _, kv := range cliOverrides {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		flags[kv[:idx]] = kv[idx+1:]
	}
	return Resolve(flags, ShellVars(os.Environ()), expanded), nil
}

// Resolver detects the current runtime environment (os, distro, shell).
type Resolver struct{}

// NewResolver returns a Resolver ready to detect the current environment.
func NewResolver() *Resolver { return &Resolver{} }

// Resolve returns a map of detected keys (os, distro, shell), applying overrides
// on top. A nil overrides map is safe.
func (r *Resolver) Resolve(overrides map[string]string) (map[string]string, error) {
	out := make(map[string]string)
	if v, err := detectOS(); err == nil {
		out["os"] = v
	}
	if v, err := detectDistro(); err == nil {
		out["distro"] = v
	}
	if v, err := detectShell(); err == nil {
		out["shell"] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out, nil
}

func detectOS() (string, error) {
	if runtime.GOOS == "darwin" {
		return "macos", nil
	}
	return runtime.GOOS, nil
}

func detectDistro() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "macos", nil
	case "linux":
		return readOSReleaseID()
	default:
		return runtime.GOOS, nil
	}
}

func readOSReleaseID() (string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("env: open /etc/os-release: %w", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			val := strings.TrimPrefix(line, "ID=")
			val = strings.Trim(val, `"`)
			return strings.ToLower(val), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("env: read /etc/os-release: %w", err)
	}
	return "", fmt.Errorf("env: ID not found in /etc/os-release")
}

func detectShell() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "", fmt.Errorf("env: $SHELL not set")
	}
	return strings.ToLower(filepath.Base(shell)), nil
}
