package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShellConfig holds the shell name and its RC file path.
type ShellConfig struct {
	Shell  string
	RCFile string
}

// DetectShellConfig returns the RC file for the given shell and OS.
// shell is the basename (bash, zsh, fish). osName is "macos" or "linux".
// Returns ok=false for unrecognized shells.
func DetectShellConfig(shell, osName string) (ShellConfig, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ShellConfig{}, false, fmt.Errorf("setup: home dir: %w", err)
	}
	switch shell {
	case "bash":
		rc := filepath.Join(home, ".bashrc")
		if osName == "macos" {
			rc = filepath.Join(home, ".bash_profile")
		}
		return ShellConfig{Shell: "bash", RCFile: rc}, true, nil
	case "zsh":
		return ShellConfig{Shell: "zsh", RCFile: filepath.Join(home, ".zshrc")}, true, nil
	case "fish":
		return ShellConfig{Shell: "fish", RCFile: filepath.Join(home, ".config", "fish", "config.fish")}, true, nil
	default:
		return ShellConfig{}, false, nil
	}
}

// SourceLine returns the shell source line for initPath expressed as $HOME-relative.
// Falls back to an absolute path if initPath is not under $HOME.
func SourceLine(initPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("setup: home dir: %w", err)
	}
	rel, err := filepath.Rel(home, initPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Sprintf(`source "%s"`, initPath), nil
	}
	return fmt.Sprintf(`source "$HOME/%s"`, rel), nil
}

// HasSourceLine reports whether rcFile already contains a source line referencing initPath.
// Returns false (not an error) if rcFile does not exist.
func HasSourceLine(rcFile, initPath string) (bool, error) {
	f, err := os.Open(rcFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setup: open %s: %w", rcFile, err)
	}
	defer func() { _ = f.Close() }()

	// Look for any source line that references the init file by name.
	needle := filepath.Base(initPath)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "source") && strings.Contains(line, needle) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// AppendSourceLine appends the dotd source line for initPath to rcFile.
// Creates rcFile if it does not exist.
func AppendSourceLine(rcFile, initPath string) error {
	line, err := SourceLine(initPath)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", rcFile, err)
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintf(f, "\n# dotd — generated shell init\n%s\n", line)
	return err
}
