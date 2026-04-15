package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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
