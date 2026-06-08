package main

import (
	"testing"

	"github.com/rocne/dot-dagger/internal/packages"
)

func TestUniquePackages(t *testing.T) {
	reqs := []packages.PackageRequest{
		{Package: "git", Hard: true},
		{Package: "git", Hard: false}, // duplicate
		{Package: "vim", Hard: false},
		{Package: "vim", Hard: true}, // duplicate
		{Package: "curl", Hard: true},
	}

	got := uniquePackages(reqs)

	if len(got) != 3 {
		t.Fatalf("want 3 unique packages, got %d: %v", len(got), got)
	}
	names := map[string]bool{}
	for _, r := range got {
		if names[r.Package] {
			t.Errorf("duplicate package %q in result", r.Package)
		}
		names[r.Package] = true
	}
	// first occurrence wins
	if got[0].Package != "git" || !got[0].Hard {
		t.Errorf("first entry should be git/Hard=true, got %+v", got[0])
	}
}

func TestUniquePackages_empty(t *testing.T) {
	if got := uniquePackages(nil); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}
