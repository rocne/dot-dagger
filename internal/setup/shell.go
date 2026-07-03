// Package setup manages shell RC file integration (source-line install/remove).
package setup

import (
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

// isGeneratedSourceLine reports whether line is exactly the kind of source line
// AppendSourceLine writes for initPath — i.e. `source "<path>"` where <path>
// resolves to initPath. This is a structural match, not a substring match: a
// comment that merely mentions the basename, or an unrelated `source` line that
// only shares the basename, must not match.
func isGeneratedSourceLine(line, initPath string) bool {
	s := strings.TrimSpace(line)
	const prefix = `source "`
	if len(s) <= len(prefix) || !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, `"`) {
		return false
	}
	path := s[len(prefix) : len(s)-1]
	// Absolute fallback form: SourceLine emits the raw path when initPath is not
	// under $HOME.
	if path == initPath {
		return true
	}
	// $HOME-relative form: SourceLine emits `$HOME/<rel>` where rel is initPath
	// relative to home. We don't have home here, but this line only appears in
	// dotd's own header-anchored block, so matching the basename of the
	// $HOME-relative path is sufficient to distinguish it from an absolute or
	// tilde-prefixed line for a different file.
	if rel, ok := strings.CutPrefix(path, "$HOME/"); ok {
		return filepath.Base(rel) == filepath.Base(initPath)
	}
	return false
}

// HasSourceLine reports whether rcFile already contains dotd's generated block
// for initPath. Returns false (not an error) if rcFile does not exist.
//
// dotd owns a two-line block: the header sentinel (sourceLineHeader) immediately
// followed by its generated source line. Detection matches that block
// structurally — never a bare line that merely mentions "source" or happens to
// share the init file's basename.
func HasSourceLine(rcFile, initPath string) (bool, error) {
	f, err := os.Open(rcFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setup: open %s: %w", rcFile, err)
	}
	defer func() { _ = f.Close() }()

	scanner := fileutil.NewLineScanner(f)
	sawHeader := false
	for scanner.Scan() {
		line := scanner.Text()
		if sawHeader && isGeneratedSourceLine(line, initPath) {
			return true, nil
		}
		sawHeader = line == sourceLineHeader
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

	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if lines[i] == sourceLineHeader {
			// AppendSourceLine writes a leading blank line before the header;
			// drop it too so add→remove is an exact identity and repeated
			// init→teardown cycles never accrete blank lines.
			if n := len(out); n > 0 && out[n-1] == "" {
				out = out[:n-1]
			}
			i++ // skip header
			// skip the following source line if it is dotd's generated line
			if i < len(lines) && isGeneratedSourceLine(lines[i], initFile) {
				i++
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}

	// Rewrite atomically (temp file + rename) so an interrupted write can never
	// truncate or corrupt a file dotd does not own. Preserve the existing mode.
	mode := fileutil.ModeFile
	if info, statErr := os.Stat(rcFile); statErr == nil {
		mode = info.Mode().Perm()
	}
	return fileutil.WriteAtomic(rcFile, []byte(strings.Join(out, "\n")), mode)
}
