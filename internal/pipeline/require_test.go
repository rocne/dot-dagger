package pipeline

import (
	"errors"
	"testing"

	"github.com/rocne/dot-dagger/internal/packages"
)

// fakeLookPath resolves only the names in found; anything else errors, like
// exec.LookPath on a binary not on PATH.
func fakeLookPath(found ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, f := range found {
		set[f] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestCheckRequirements_Installed_NotUnmet(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "a", Require: []string{"sh"}},
	}
	got, err := CheckRequirements(nodes, nil, fakeLookPath("sh"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no unmet requires, got %v", got)
	}
}

func TestCheckRequirements_Installable_NotUnmet(t *testing.T) {
	reg := &packages.Registry{
		PackageManagers: packages.ManagersSection{
			Order: []string{"brew"},
			Defs:  map[string]packages.PackageManagerDef{"brew": {Install: "brew install {package}"}},
		},
		Packages: map[string]packages.PackageEntry{
			"ripgrep": {Managers: map[string]packages.ManagerEntry{"brew": {}}},
		},
	}
	nodes := []RawNode{
		{LogicalName: "a", Require: []string{"ripgrep"}},
	}
	// ripgrep itself isn't on PATH, but brew (its manager) is.
	got, err := CheckRequirements(nodes, reg, fakeLookPath("brew"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no unmet requires (installable via brew), got %v", got)
	}
}

func TestCheckRequirements_NeitherInstalledNorInstallable_Unmet(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "shellrc.req-test", Require: []string{"nonexistent-pkg-xyz"}},
	}
	got, err := CheckRequirements(nodes, nil, fakeLookPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unmet require, got %v", got)
	}
	if got[0].Node != "shellrc.req-test" || got[0].Package != "nonexistent-pkg-xyz" {
		t.Errorf("unexpected UnmetRequire: %+v", got[0])
	}
}

func TestCheckRequirements_NilRegistry_PathOnly(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "a", Require: []string{"sh"}},
		{LogicalName: "b", Require: []string{"missing-tool"}},
	}
	got, err := CheckRequirements(nodes, nil, fakeLookPath("sh"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Node != "b" || got[0].Package != "missing-tool" {
		t.Errorf("expected only b/missing-tool unmet, got %v", got)
	}
}

func TestCheckRequirements_MultipleRequiresOnOneNode(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "a", Require: []string{"sh", "missing-one", "missing-two"}},
	}
	got, err := CheckRequirements(nodes, nil, fakeLookPath("sh"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unmet requires, got %v", got)
	}
	pkgs := map[string]bool{got[0].Package: true, got[1].Package: true}
	if !pkgs["missing-one"] || !pkgs["missing-two"] {
		t.Errorf("expected missing-one and missing-two, got %v", got)
	}
}

func TestCheckRequirements_NoRequire_Empty(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "a"},
	}
	got, err := CheckRequirements(nodes, nil, fakeLookPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
