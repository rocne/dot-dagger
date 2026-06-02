package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestGetOSCmd verifies the hidden get-os command outputs the normalized OS name.
// On darwin, it normalizes to "macos"; otherwise it lowercases runtime.GOOS.
func TestGetOSCmd(t *testing.T) {
	out, err := run(t, "get-os")
	if err != nil {
		t.Fatalf("get-os error = %v", err)
	}
	got := strings.TrimSpace(out)
	want := strings.ToLower(runtime.GOOS)
	if runtime.GOOS == "darwin" {
		want = "macos"
	}
	if got != want {
		t.Errorf("get-os = %q, want %q", got, want)
	}
}

// TestGetHostnameCmd verifies the hidden get-hostname command outputs os.Hostname().
func TestGetHostnameCmd(t *testing.T) {
	out, err := run(t, "get-hostname")
	if err != nil {
		t.Fatalf("get-hostname error = %v", err)
	}
	got := strings.TrimSpace(out)
	want, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname: %v", err)
	}
	if got != want {
		t.Errorf("get-hostname = %q, want %q", got, want)
	}
}
