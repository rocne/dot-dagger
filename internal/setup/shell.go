// Package setup manages shell RC file integration (source-line install/remove).
package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
)

// sourceLineHeader is the comment line written above the RC source line.
// AppendSourceLine writes it; RemoveSourceLine matches it — one constant so
// the two can never drift.
const sourceLineHeader = "# dotd — generated shell init"

// ShellConfig holds the shell name and its RC file path.
type ShellConfig struct {
	Shell  string
	RCFile string
}

// DetectShellConfig returns the RC file for the given shell and OS.
// shell is the basename (bash, zsh, fish). osName is "macos" or "linux".
// home is the user's home directory (use cfg.home from the caller).
// configDir is the XDG config home (use cfg.configDir from the caller); only
// fish needs it. Returns ok=false for unrecognized shells.
func DetectShellConfig(shell, osName, home, configDir string) (ShellConfig, bool) {
	switch shell {
	case "bash":
		rc := filepath.Join(home, ".bashrc")
		if osName == "macos" {
			rc = filepath.Join(home, ".bash_profile")
		}
		return ShellConfig{Shell: "bash", RCFile: rc}, true
	case "zsh":
		return ShellConfig{Shell: "zsh", RCFile: filepath.Join(home, ".zshrc")}, true
	case "fish":
		return ShellConfig{Shell: "fish", RCFile: filepath.Join(configDir, "fish", "config.fish")}, true
	default:
		return ShellConfig{}, false
	}
}

// SourceLine returns the shell source line for initPath expressed as $HOME-relative.
// home is the user's home directory (use cfg.home from the caller).
// Falls back to an absolute path if initPath is not under home.
func SourceLine(initPath, home string) (string, error) {
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
	if errors.Is(err, fs.ErrNotExist) {
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
// home is the user's home directory (use cfg.home from the caller).
// Creates rcFile if it does not exist.
func AppendSourceLine(rcFile, initPath, home string) error {
	line, err := SourceLine(initPath, home)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileutil.ModeFile)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", rcFile, err)
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintf(f, "\n%s\n%s\n", sourceLineHeader, line)
	return err
}

// RemoveSourceLine removes the dotd source line and its comment header from rcFile.
// The header it matches is the shared sourceLineHeader const (so it can never
// drift from what AppendSourceLine writes).
// No-op if rcFile does not exist or the lines are not present.
func RemoveSourceLine(rcFile, initFile string) error {
	data, err := os.ReadFile(rcFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("setup: read %s: %w", rcFile, err)
	}

	lines := strings.Split(string(data), "\n")
	needle := filepath.Base(initFile)

	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if lines[i] == sourceLineHeader {
			i++ // skip header
			// skip the following source line if it references our init file
			if i < len(lines) && strings.Contains(lines[i], "source") && strings.Contains(lines[i], needle) {
				i++
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}

	return os.WriteFile(rcFile, []byte(strings.Join(out, "\n")), fileutil.ModeFile)
}
