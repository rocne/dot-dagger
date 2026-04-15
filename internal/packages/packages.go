// Package packages manages the package registry and provides the installed()
// and installable() predicate functions for the dotp tool.
//
// The registry is loaded from packages.yaml at the dotfiles repo root.
// Package entries have known fields (binary, check, prefer) plus dynamic per-manager
// entries whose keys are package manager names.
package packages

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
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
	// Defs maps manager names to their command templates.
	Defs map[string]PackageManagerDef
}

// UnmarshalYAML implements yaml.Unmarshaler for ManagersSection.
// Extracts the known field "priority" and treats all other keys as manager defs.
func (ms *ManagersSection) UnmarshalYAML(value *yaml.Node) error {
	ms.Defs = make(map[string]PackageManagerDef)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if key == "priority" {
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

	known := map[string]bool{"binary": true, "check": true, "prefer": true}
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
	PackageManagers ManagersSection        `yaml:"package_managers"`
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

// LoadFile loads a registry from a file path.
// Returns an empty registry if the file does not exist.
func LoadFile(path string) (*Registry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Registry{
			PackageManagers: ManagersSection{Defs: map[string]PackageManagerDef{}},
			Packages:        map[string]PackageEntry{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("packages: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}

// ManagerOrder returns the ordered list of managers to try for a package.
// Resolution: per-package prefer → global priority → catalog order of the package's managers.
func ManagerOrder(pkg string, reg *Registry) []string {
	if entry, ok := reg.Packages[pkg]; ok && len(entry.Prefer) > 0 {
		return entry.Prefer
	}
	if len(reg.PackageManagers.Priority) > 0 {
		return reg.PackageManagers.Priority
	}
	// Fallback: managers the package has entries for, in catalog order.
	entry, ok := reg.Packages[pkg]
	if !ok {
		return nil
	}
	var order []string
	for _, m := range Catalog {
		if _, has := entry.Managers[m.Name]; has {
			order = append(order, m.Name)
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

// Installed returns true if the package's binary is present on PATH.
// lookPath should be exec.LookPath for production; injected for testing.
func Installed(pkg string, reg *Registry, lookPath func(string) (string, error)) (bool, error) {
	bin := BinaryName(pkg, reg)
	_, err := lookPath(bin)
	if err == nil {
		return true, nil
	}
	// Any error from LookPath means "not found" — not an error we propagate.
	return false, nil
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

// PackageRequest records a package requirement from a file annotation.
type PackageRequest struct {
	// Package is the logical package name.
	Package string
	// Hard is true for @require (gates file), false for @request (soft ask).
	Hard bool
	// NodePath is the source file declaring the requirement.
	NodePath string
}

// CollectRequests gathers @require and @request annotations from all nodes.
func CollectRequests(nodes []fileset.Node) []PackageRequest {
	var reqs []PackageRequest
	for _, n := range nodes {
		for _, a := range annotation.Get(n.Annotations, annotation.KeyRequire) {
			if a.Value != "" {
				reqs = append(reqs, PackageRequest{
					Package:  strings.TrimSpace(a.Value),
					Hard:     true,
					NodePath: n.Path,
				})
			}
		}
		for _, a := range annotation.Get(n.Annotations, annotation.KeyRequest) {
			if a.Value != "" {
				reqs = append(reqs, PackageRequest{
					Package:  strings.TrimSpace(a.Value),
					Hard:     false,
					NodePath: n.Path,
				})
			}
		}
	}
	return reqs
}

// InstallCmd returns the install command for a package using the given manager.
// Returns an error if the manager or its command template is not found in the registry.
func InstallCmd(pkg, manager string, reg *Registry) (string, error) {
	mgDef, ok := reg.PackageManagers.Defs[manager]
	if !ok {
		return "", fmt.Errorf("packages: unknown package manager %q", manager)
	}

	// Per-package install command override.
	if entry, ok := reg.Packages[pkg]; ok {
		if me, ok := entry.Managers[manager]; ok && me.Install != "" {
			return me.Install, nil
		}
	}

	if mgDef.Install == "" {
		return "", fmt.Errorf("packages: no install command for manager %q", manager)
	}

	// Substitute {package}.
	pkgName := packageName(pkg, manager, reg)
	cmd := strings.ReplaceAll(mgDef.Install, "{package}", pkgName)
	return cmd, nil
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
