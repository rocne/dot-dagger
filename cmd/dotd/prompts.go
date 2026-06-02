package main

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/ui"
)

// promptConfirm prints "Proceed? [y/N]: ", reads a line, and returns true only
// on "y" or "yes". Any other input (including empty / Enter) prints "cancelled"
// and returns false — callers should return nil when false.
func promptConfirm(out io.Writer, r io.Reader) bool {
	fmt.Fprint(out, "\nProceed? [y/N]: ")
	ans, _ := bufio.NewReader(r).ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans != "y" && ans != "yes" {
		ui.Skipf(out, "cancelled")
		return false
	}
	return true
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, ui.Skip(defaultVal))
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// promptYN prints "msg [Y/n]: " and returns true unless user types n/no.
// Empty input and EOF both default to yes.
func promptYN(w io.Writer, reader *bufio.Reader, msg string) (bool, error) {
	fmt.Fprintf(w, "  %s [Y/n]: ", msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		return true, nil // EOF → default yes
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes", nil
}

func expandTildeStr(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// printField prints a bold field label and a faint description, then a blank line.
func printField(w io.Writer, label, desc string) {
	fmt.Fprintf(w, "\n  %s\n", ui.Key(label))
	fmt.Fprintf(w, "  %s\n", ui.Skip(desc))
}

// fieldPrompt returns the prompt text used after a printField call.
func fieldPrompt() string {
	return "  " + ui.Arrow("›")
}
