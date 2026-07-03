// Package packages manages the package registry and provides the installed()
// and installable() predicate functions for dotd package management.
//
// The registry is loaded from packages.yaml at the dotfiles repo root.
// Package entries have known fields (binary, check, prefer) plus dynamic per-manager
// entries whose keys are package manager names.
package packages

import (
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

// checkRunner executes a custom shell expression and reports whether the command
// exits successfully. Overridable in tests via a package-level variable.
var checkRunner = func(expr string) (bool, error) {
	cmd := exec.Command("sh", "-c", expr)
	return cmd.Run() == nil, nil
}

// PlaceholderToken is the substitution token replaced with the package name in
// command templates. Every Install/Uninstall/Update template must contain this token.
const PlaceholderToken = "{package}"

// Reserved key names for PackageEntry fields. These are parsed out of the YAML
// map before treating remaining keys as package-manager entries.
const (
	keyPriority = "priority"
	keyBinary   = "binary"
	keyCheck    = "check"
	keyPrefer   = "prefer"
)

// PackageManagerDef defines the command templates for a package manager.
// {package} is substituted with the package name at runtime.
type PackageManagerDef struct {
	Install   string `yaml:"install"`
	Uninstall string `yaml:"uninstall"`
	Update    string `yaml:"update"`
}

// ManagerEntry is a per-package override for a specific package manager.
// Empty string fields mean "use the package manager default".
type ManagerEntry struct {
	// Package overrides the package name passed to this manager.
	Package string `yaml:"package"`
	// Install, Uninstall, Update override the global command templates.
	Install   string `yaml:"install"`
	Uninstall string `yaml:"uninstall"`
	Update    string `yaml:"update"`
}

// ManagersSection holds the package_managers block from packages.yaml.
// It contains an optional priority list and per-manager command templates.
type ManagersSection struct {
	// Priority is the global preference order for package managers.
	// When multiple managers can install a package, the first one on PATH wins.
	Priority []string
	// Order preserves the YAML declaration order of manager definitions.
	Order []string
	// Defs maps manager names to their command templates.
	Defs map[string]PackageManagerDef
}

// UnmarshalYAML implements yaml.Unmarshaler for ManagersSection.
// Extracts the known field "priority" and treats all other keys as manager defs.
func (ms *ManagersSection) UnmarshalYAML(value *yaml.Node) error {
	ms.Defs = make(map[string]PackageManagerDef)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if key == keyPriority {
			if err := value.Content[i+1].Decode(&ms.Priority); err != nil {
				return fmt.Errorf("packages: decode priority: %w", err)
			}
			continue
		}
		var def PackageManagerDef
		if err := value.Content[i+1].Decode(&def); err != nil {
			return fmt.Errorf("packages: decode manager %q: %w", key, err)
		}
		ms.Defs[key] = def
		ms.Order = append(ms.Order, key)
	}
	return nil
}

// PackageEntry describes a logical package in the registry.
type PackageEntry struct {
	// Binary is the executable name to check for installed(). Defaults to the package name.
	Binary string
	// Check is a custom shell expression to test for installation. Defaults to "which {binary}".
	Check string
	// Prefer overrides the global manager priority for this package only.
	Prefer []string
	// Managers maps package manager names to their per-package override entries.
	Managers map[string]ManagerEntry
}

// UnmarshalYAML implements yaml.Unmarshaler.
// It extracts the known fields (binary, check, prefer) and treats all other keys as manager entries.
func (e *PackageEntry) UnmarshalYAML(value *yaml.Node) error {
	type knownFields struct {
		Binary string   `yaml:"binary"`
		Check  string   `yaml:"check"`
		Prefer []string `yaml:"prefer"`
	}
	var kf knownFields
	if err := value.Decode(&kf); err != nil {
		return err
	}
	e.Binary = kf.Binary
	e.Check = kf.Check
	e.Prefer = kf.Prefer
	e.Managers = make(map[string]ManagerEntry)

	known := map[string]bool{keyBinary: true, keyCheck: true, keyPrefer: true}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if known[key] {
			continue
		}
		var me ManagerEntry
		if err := value.Content[i+1].Decode(&me); err != nil {
			return fmt.Errorf("packages: decode manager entry %q: %w", key, err)
		}
		e.Managers[key] = me
	}
	return nil
}

// Registry is the parsed packages.yaml.
type Registry struct {
	PackageManagers ManagersSection         `yaml:"package_managers"`
	Packages        map[string]PackageEntry `yaml:"packages"`
}

// Load parses a packages.yaml registry from r.
func Load(r io.Reader) (*Registry, error) {
	var reg Registry
	if err := yaml.NewDecoder(r).Decode(&reg); err != nil {
		return nil, fmt.Errorf("packages: parse registry: %w", err)
	}
	return &reg, nil
}

// EmptyRegistry returns a registry with no entries and non-nil maps, safe for
// all lookups. The canonical "no packages.yaml" value — LoadFile and the
// predicate built-ins both use it.
func EmptyRegistry() *Registry {
	return &Registry{
		PackageManagers: ManagersSection{Defs: map[string]PackageManagerDef{}},
		Packages:        map[string]PackageEntry{},
	}
}

// LoadFile loads a registry from a file path.
// Returns an empty registry if the file does not exist.
func LoadFile(path string) (*Registry, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return EmptyRegistry(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("packages: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}

// ManagerOrder returns the ordered list of managers to try for a package.
// Resolution: per-package prefer → global priority → packages.yaml declaration order.
func ManagerOrder(pkg string, reg *Registry) []string {
	if entry, ok := reg.Packages[pkg]; ok && len(entry.Prefer) > 0 {
		return entry.Prefer
	}
	if len(reg.PackageManagers.Priority) > 0 {
		return reg.PackageManagers.Priority
	}
	// Fallback: managers the package supports, in packages.yaml declaration order.
	entry, ok := reg.Packages[pkg]
	if !ok {
		return nil
	}
	var order []string
	for _, m := range reg.PackageManagers.Order {
		if _, has := entry.Managers[m]; has {
			order = append(order, m)
		}
	}
	return order
}

// BinaryName returns the binary to check for a package.
// Falls back to the package name if no binary field is set.
func BinaryName(name string, reg *Registry) string {
	if entry, ok := reg.Packages[name]; ok && entry.Binary != "" {
		return entry.Binary
	}
	return name
}

// Installed returns true if the package is present on the system.
// If the package entry has a non-empty Check field, that shell expression is
// run via checkRunner (defaults to exec "sh -c <expr>"); a zero exit means
// installed. Otherwise falls back to lookPath on the binary name.
// lookPath should be exec.LookPath for production; injected for testing.
func Installed(pkg string, reg *Registry, lookPath func(string) (string, error)) (bool, error) {
	if entry, ok := reg.Packages[pkg]; ok && entry.Check != "" {
		return checkRunner(entry.Check)
	}
	bin := BinaryName(pkg, reg)
	_, err := lookPath(bin)
	// Any error from LookPath means "not found" — not an error we propagate.
	return err == nil, nil
}

// Installable returns true if the registry has an entry for pkg with at least
// one manager in its resolved order that is present on PATH.
func Installable(pkg string, reg *Registry, lookPath func(string) (string, error)) (bool, error) {
	entry, ok := reg.Packages[pkg]
	if !ok {
		return false, nil
	}
	for _, mgr := range ManagerOrder(pkg, reg) {
		if _, hasEntry := entry.Managers[mgr]; !hasEntry {
			continue
		}
		if _, err := lookPath(mgr); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// PackageRequest records a package requirement from a file or manifest.
type PackageRequest struct {
	Package  string // logical package name
	Hard     bool   // true for @require (hard gate), false for @request (soft)
	NodePath string // source file declaring the requirement
}

// InstallCmd returns the install command for a package using the given manager.
// Returns an error if the manager or its command template is not found in the registry.
func InstallCmd(pkg, manager string, reg *Registry) (string, error) {
	mgDef, ok := reg.PackageManagers.Defs[manager]
	if !ok {
		return "", fmt.Errorf("packages: unknown package manager %q", manager)
	}

	pkgName := packageName(pkg, manager, reg)

	// Per-package install command override — substituted like the global
	// template, so an override may also use {package}.
	if entry, ok := reg.Packages[pkg]; ok {
		if me, ok := entry.Managers[manager]; ok && me.Install != "" {
			return strings.ReplaceAll(me.Install, PlaceholderToken, pkgName), nil
		}
	}

	if mgDef.Install == "" {
		return "", fmt.Errorf("packages: no install command for manager %q", manager)
	}

	return strings.ReplaceAll(mgDef.Install, PlaceholderToken, pkgName), nil
}

// packageName returns the package name to pass to a manager.
// Checks the per-manager Package override, then falls back to the logical name.
func packageName(pkg, manager string, reg *Registry) string {
	if entry, ok := reg.Packages[pkg]; ok {
		if me, ok := entry.Managers[manager]; ok && me.Package != "" {
			return me.Package
		}
	}
	return pkg
}

// resolveInstallCmd returns the install command for a package, selecting the
// first manager in priority order that is present on PATH.
func resolveInstallCmd(pkg string, reg *Registry, lookPath func(string) (string, error)) (string, error) {
	entry, ok := reg.Packages[pkg]
	if !ok {
		return "", fmt.Errorf("packages: %q not in registry", pkg)
	}
	for _, m := range ManagerOrder(pkg, reg) {
		if _, hasEntry := entry.Managers[m]; !hasEntry {
			continue
		}
		if _, err := lookPath(m); err != nil {
			continue
		}
		return InstallCmd(pkg, m, reg)
	}
	return "", fmt.Errorf("packages: no manager on PATH for %q", pkg)
}

// GenerateScript writes a POSIX shell script that installs all required packages.
// Packages already installed (per lookPath) are skipped.
// Returns an error if a hard requirement (@require) cannot be satisfied.
func GenerateScript(w io.Writer, reqs []PackageRequest, reg *Registry, lookPath func(string) (string, error)) error {
	fmt.Fprintln(w, fileutil.POSIXShebang)
	fmt.Fprintln(w, ecosystem.GeneratedFileHeader())
	fmt.Fprintln(w, "# Review before running — some package managers need root.")
	fmt.Fprintln(w)

	seen := make(map[string]bool)
	for _, req := range reqs {
		if seen[req.Package] {
			continue
		}
		seen[req.Package] = true

		ok, err := Installed(req.Package, reg, lookPath)
		if err != nil {
			return err
		}
		if ok {
			continue
		}

		cmd, err := resolveInstallCmd(req.Package, reg, lookPath)
		if err != nil {
			if req.Hard {
				return fmt.Errorf("packages: @require %q (from %s): %w", req.Package, req.NodePath, err)
			}
			continue
		}
		fmt.Fprintln(w, cmd)
	}
	return nil
}
