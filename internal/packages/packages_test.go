package packages

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
)

const sampleYAML = `
package_managers:
  priority: [brew, apt]
  brew:
    install: brew install {package}
    uninstall: brew uninstall {package}
    update: brew upgrade {package}
  apt:
    install: apt install -y {package}
    uninstall: apt remove -y {package}
    update: apt upgrade -y {package}

packages:
  fzf:
    brew: {}
    apt: {}

  ripgrep:
    binary: rg
    brew: {}
    apt: {}

  python-dateutil:
    pip:
      package: python-dateutil
    apt:
      package: python3-dateutil

  some-tool:
    brew:
      install: brew tap someorg/sometool && brew install some-tool
    apt: {}

  yum-preferred:
    prefer: [yum, apt]
    yum: {}
    apt: {}
`

func loadSample(t *testing.T) *Registry {
	t.Helper()
	reg, err := Load(strings.NewReader(sampleYAML))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return reg
}

// --- Load / parse ---

func TestLoadParseManagers(t *testing.T) {
	reg := loadSample(t)
	if len(reg.PackageManagers.Defs) != 2 {
		t.Errorf("PackageManagers.Defs len = %d, want 2", len(reg.PackageManagers.Defs))
	}
	brew := reg.PackageManagers.Defs["brew"]
	if brew.Install != "brew install {package}" {
		t.Errorf("brew.Install = %q", brew.Install)
	}
}

func TestLoadParsePriority(t *testing.T) {
	reg := loadSample(t)
	if len(reg.PackageManagers.Priority) != 2 {
		t.Fatalf("Priority len = %d, want 2", len(reg.PackageManagers.Priority))
	}
	if reg.PackageManagers.Priority[0] != "brew" {
		t.Errorf("Priority[0] = %q, want brew", reg.PackageManagers.Priority[0])
	}
}

func TestLoadParsePackages(t *testing.T) {
	reg := loadSample(t)
	if len(reg.Packages) != 5 {
		t.Errorf("Packages len = %d, want 5", len(reg.Packages))
	}
}

func TestLoadBinaryField(t *testing.T) {
	reg := loadSample(t)
	if reg.Packages["ripgrep"].Binary != "rg" {
		t.Errorf("ripgrep binary = %q, want rg", reg.Packages["ripgrep"].Binary)
	}
}

func TestLoadManagerEntryPackageOverride(t *testing.T) {
	reg := loadSample(t)
	entry := reg.Packages["python-dateutil"]
	if entry.Managers["apt"].Package != "python3-dateutil" {
		t.Errorf("apt package = %q, want python3-dateutil", entry.Managers["apt"].Package)
	}
}

func TestLoadManagerInstallOverride(t *testing.T) {
	reg := loadSample(t)
	entry := reg.Packages["some-tool"]
	if entry.Managers["brew"].Install != "brew tap someorg/sometool && brew install some-tool" {
		t.Errorf("brew install = %q", entry.Managers["brew"].Install)
	}
}

func TestLoadEmptyManagerEntry(t *testing.T) {
	reg := loadSample(t)
	// fzf has brew: {} — empty ManagerEntry, no overrides.
	entry := reg.Packages["fzf"]
	if _, ok := entry.Managers["brew"]; !ok {
		t.Error("fzf brew manager entry missing")
	}
	if entry.Managers["brew"].Package != "" {
		t.Error("fzf brew.Package should be empty")
	}
}

func TestLoadPreferField(t *testing.T) {
	reg := loadSample(t)
	entry := reg.Packages["yum-preferred"]
	if len(entry.Prefer) != 2 || entry.Prefer[0] != "yum" {
		t.Errorf("prefer = %v, want [yum apt]", entry.Prefer)
	}
}

// --- ManagerOrder ---

func TestManagerOrderPerPackagePrefer(t *testing.T) {
	reg := loadSample(t)
	order := ManagerOrder("yum-preferred", reg)
	if len(order) == 0 || order[0] != "yum" {
		t.Errorf("ManagerOrder = %v, want yum first", order)
	}
}

func TestManagerOrderGlobalPriority(t *testing.T) {
	reg := loadSample(t)
	order := ManagerOrder("fzf", reg)
	if len(order) == 0 || order[0] != "brew" {
		t.Errorf("ManagerOrder = %v, want brew first (global priority)", order)
	}
}

func TestManagerOrderRegistryFallback(t *testing.T) {
	// Registry with no priority and no per-package prefer — falls back to declaration order.
	reg, err := Load(strings.NewReader(`
package_managers:
  apt:
    install: apt install -y {package}
packages:
  fzf:
    apt: {}
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	order := ManagerOrder("fzf", reg)
	if len(order) == 0 {
		t.Error("expected fallback order, got empty")
	}
	if order[0] != "apt" {
		t.Errorf("ManagerOrder = %v, want [apt]", order)
	}
}

// --- BinaryName ---

func TestBinaryNameDefaultsToPackageName(t *testing.T) {
	reg := loadSample(t)
	if BinaryName("fzf", reg) != "fzf" {
		t.Errorf("BinaryName(fzf) = %q, want fzf", BinaryName("fzf", reg))
	}
}

func TestBinaryNameOverride(t *testing.T) {
	reg := loadSample(t)
	if BinaryName("ripgrep", reg) != "rg" {
		t.Errorf("BinaryName(ripgrep) = %q, want rg", BinaryName("ripgrep", reg))
	}
}

func TestBinaryNameUnknownPackage(t *testing.T) {
	reg := loadSample(t)
	if BinaryName("unknown-tool", reg) != "unknown-tool" {
		t.Error("unknown package should fall back to package name")
	}
}

// --- Installed ---

func lookPathFound(bin string) (string, error) { return "/usr/bin/" + bin, nil }
func lookPathNotFound(_ string) (string, error) { return "", fmt.Errorf("not found") }

func TestInstalledTrue(t *testing.T) {
	reg := loadSample(t)
	ok, err := Installed("fzf", reg, lookPathFound)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !ok {
		t.Error("expected installed = true")
	}
}

func TestInstalledFalse(t *testing.T) {
	reg := loadSample(t)
	ok, err := Installed("fzf", reg, lookPathNotFound)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Error("expected installed = false")
	}
}

func TestInstalledUsesCustomBinary(t *testing.T) {
	reg := loadSample(t)
	// ripgrep binary is "rg" — lookPath should receive "rg".
	var received string
	ok, err := Installed("ripgrep", reg, func(bin string) (string, error) {
		received = bin
		return "/usr/bin/rg", nil
	})
	if err != nil || !ok {
		t.Fatalf("unexpected: ok=%v err=%v", ok, err)
	}
	if received != "rg" {
		t.Errorf("lookPath called with %q, want rg", received)
	}
}

// --- Installable ---

func TestInstallableTrue(t *testing.T) {
	reg := loadSample(t)
	// fzf has brew entry; global priority=[brew,apt]; brew is on PATH.
	ok, err := Installable("fzf", reg, func(bin string) (string, error) {
		if bin == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("not found")
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !ok {
		t.Error("expected installable = true")
	}
}

func TestInstallableFalseNoManager(t *testing.T) {
	reg := loadSample(t)
	// fzf has no cargo entry; cargo on PATH but not in fzf.managers.
	// Override priority via a registry with cargo as priority.
	reg.PackageManagers.Priority = []string{"cargo"}
	ok, err := Installable("fzf", reg, lookPathFound)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Error("expected installable = false (no cargo entry for fzf)")
	}
}

func TestInstallableFalseManagerNotOnPath(t *testing.T) {
	reg := loadSample(t)
	ok, err := Installable("fzf", reg, lookPathNotFound)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Error("expected installable = false (managers not on PATH)")
	}
}

func TestInstallableUnknownPackage(t *testing.T) {
	reg := loadSample(t)
	ok, err := Installable("ghost-tool", reg, lookPathFound)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if ok {
		t.Error("expected installable = false for unknown package")
	}
}

func TestInstallableUsesPerPackagePrefer(t *testing.T) {
	reg := loadSample(t)
	// yum-preferred has prefer:[yum,apt]; only yum on PATH.
	ok, err := Installable("yum-preferred", reg, func(bin string) (string, error) {
		if bin == "yum" {
			return "/usr/bin/yum", nil
		}
		return "", fmt.Errorf("not found")
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !ok {
		t.Error("expected installable = true via per-package prefer")
	}
}

// --- InstallCmd ---

func TestInstallCmdDefault(t *testing.T) {
	reg := loadSample(t)
	cmd, err := InstallCmd("fzf", "brew", reg)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if cmd != "brew install fzf" {
		t.Errorf("cmd = %q, want \"brew install fzf\"", cmd)
	}
}

func TestInstallCmdPackageOverride(t *testing.T) {
	reg := loadSample(t)
	cmd, err := InstallCmd("python-dateutil", "apt", reg)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if cmd != "apt install -y python3-dateutil" {
		t.Errorf("cmd = %q, want \"apt install -y python3-dateutil\"", cmd)
	}
}

func TestInstallCmdInstallOverride(t *testing.T) {
	reg := loadSample(t)
	cmd, err := InstallCmd("some-tool", "brew", reg)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "brew tap someorg/sometool && brew install some-tool"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

func TestInstallCmdUnknownManager(t *testing.T) {
	reg := loadSample(t)
	_, err := InstallCmd("fzf", "cargo", reg)
	if err == nil {
		t.Error("expected error for unknown manager")
	}
}

// --- CollectRequests ---

func TestCollectRequestsRequire(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/a.sh",
			Annotations: []annotation.Annotation{
				{Key: "require", Args:"fzf"},
			},
		},
	}
	reqs := CollectRequests(nodes)
	if len(reqs) != 1 {
		t.Fatalf("len = %d, want 1", len(reqs))
	}
	if reqs[0].Package != "fzf" || !reqs[0].Hard {
		t.Errorf("got %+v, want {Package:fzf Hard:true}", reqs[0])
	}
}

func TestCollectRequestsRequest(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/b.sh",
			Annotations: []annotation.Annotation{
				{Key: "request", Args:"ripgrep"},
			},
		},
	}
	reqs := CollectRequests(nodes)
	if len(reqs) != 1 {
		t.Fatalf("len = %d, want 1", len(reqs))
	}
	if reqs[0].Package != "ripgrep" || reqs[0].Hard {
		t.Errorf("got %+v, want {Package:ripgrep Hard:false}", reqs[0])
	}
}

func TestCollectRequestsMultipleNodes(t *testing.T) {
	nodes := []fileset.Node{
		{
			Path: "/a.sh",
			Annotations: []annotation.Annotation{
				{Key: "require", Args:"fzf"},
				{Key: "request", Args:"ripgrep"},
			},
		},
		{
			Path: "/b.sh",
			Annotations: []annotation.Annotation{
				{Key: "require", Args:"some-tool"},
			},
		},
	}
	reqs := CollectRequests(nodes)
	if len(reqs) != 3 {
		t.Fatalf("len = %d, want 3", len(reqs))
	}
}

func TestCollectRequestsEmpty(t *testing.T) {
	nodes := []fileset.Node{
		{Path: "/a.sh", Annotations: []annotation.Annotation{{Key: "when", Args:"os=linux"}}},
	}
	reqs := CollectRequests(nodes)
	if len(reqs) != 0 {
		t.Errorf("expected no requests, got %d", len(reqs))
	}
}

// --- GenerateScript ---

func TestGenerateScriptEmitsInstallCmd(t *testing.T) {
	reg := loadSample(t)
	reqs := []PackageRequest{
		{Package: "fzf", Hard: true, NodePath: "/a.sh"},
	}
	var buf bytes.Buffer
	err := GenerateScript(&buf, reqs, reg, func(bin string) (string, error) {
		if bin == "fzf" {
			return "", fmt.Errorf("not found") // not installed
		}
		if bin == "brew" {
			return "/usr/local/bin/brew", nil // manager on PATH
		}
		return "", fmt.Errorf("not found")
	})
	if err != nil {
		t.Fatalf("GenerateScript() error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "brew install fzf") {
		t.Errorf("output missing install cmd, got:\n%s", out)
	}
}

func TestGenerateScriptSkipsInstalled(t *testing.T) {
	reg := loadSample(t)
	reqs := []PackageRequest{
		{Package: "fzf", Hard: true, NodePath: "/a.sh"},
	}
	var buf bytes.Buffer
	err := GenerateScript(&buf, reqs, reg, lookPathFound) // fzf "installed"
	if err != nil {
		t.Fatalf("GenerateScript() error = %v", err)
	}
	if strings.Contains(buf.String(), "install") {
		t.Errorf("expected no install cmd for already-installed package, got:\n%s", buf.String())
	}
}

func TestGenerateScriptDeduplicates(t *testing.T) {
	reg := loadSample(t)
	reqs := []PackageRequest{
		{Package: "fzf", Hard: true, NodePath: "/a.sh"},
		{Package: "fzf", Hard: false, NodePath: "/b.sh"},
	}
	var buf bytes.Buffer
	err := GenerateScript(&buf, reqs, reg, func(bin string) (string, error) {
		if bin == "brew" {
			return "/usr/local/bin/brew", nil
		}
		return "", fmt.Errorf("not found")
	})
	if err != nil {
		t.Fatalf("GenerateScript() error = %v", err)
	}
	count := strings.Count(buf.String(), "brew install fzf")
	if count != 1 {
		t.Errorf("expected 1 install cmd, got %d", count)
	}
}

func TestGenerateScriptHardRequireError(t *testing.T) {
	reg := loadSample(t)
	reqs := []PackageRequest{
		{Package: "fzf", Hard: true, NodePath: "/a.sh"},
	}
	err := GenerateScript(&bytes.Buffer{}, reqs, reg, lookPathNotFound)
	if err == nil {
		t.Error("expected error for uninstallable @require package")
	}
}

func TestGenerateScriptSoftRequestSkipped(t *testing.T) {
	reg := loadSample(t)
	reqs := []PackageRequest{
		{Package: "fzf", Hard: false, NodePath: "/a.sh"},
	}
	var buf bytes.Buffer
	err := GenerateScript(&buf, reqs, reg, lookPathNotFound)
	if err != nil {
		t.Fatalf("GenerateScript() error = %v (want nil for @request)", err)
	}
	if strings.Contains(buf.String(), "install") {
		t.Errorf("expected no install cmd for uninstallable @request, got:\n%s", buf.String())
	}
}
