package manifest

import (
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	yaml := `
- packages:
    - ripgrep
    - fd

- when: os=macos
  packages:
    - aerospace
`
	blocks, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}

	if blocks[0].When != "" {
		t.Errorf("blocks[0].When = %q, want %q", blocks[0].When, "")
	}
	wantPkgs0 := []string{"ripgrep", "fd"}
	if len(blocks[0].Packages) != len(wantPkgs0) {
		t.Errorf("blocks[0].Packages = %v, want %v", blocks[0].Packages, wantPkgs0)
	} else {
		for i, p := range wantPkgs0 {
			if blocks[0].Packages[i] != p {
				t.Errorf("blocks[0].Packages[%d] = %q, want %q", i, blocks[0].Packages[i], p)
			}
		}
	}

	if blocks[1].When != "os=macos" {
		t.Errorf("blocks[1].When = %q, want %q", blocks[1].When, "os=macos")
	}
	wantPkgs1 := []string{"aerospace"}
	if len(blocks[1].Packages) != len(wantPkgs1) {
		t.Errorf("blocks[1].Packages = %v, want %v", blocks[1].Packages, wantPkgs1)
	} else {
		for i, p := range wantPkgs1 {
			if blocks[1].Packages[i] != p {
				t.Errorf("blocks[1].Packages[%d] = %q, want %q", i, blocks[1].Packages[i], p)
			}
		}
	}
}

func TestCollectFromPaths_unconditional(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- packages:
    - ripgrep
    - fd
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d reqs, want 2", len(reqs))
	}
	if reqs[0].Package != "ripgrep" {
		t.Errorf("reqs[0].Package = %q, want %q", reqs[0].Package, "ripgrep")
	}
	if reqs[1].Package != "fd" {
		t.Errorf("reqs[1].Package = %q, want %q", reqs[1].Package, "fd")
	}
	if reqs[0].Hard {
		t.Errorf("reqs[0].Hard = true, want false")
	}
}

func TestCollectFromPaths_predicate_match(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace

- when: os=linux
  packages:
    - i3
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("got %d reqs, want 1", len(reqs))
	}
	if reqs[0].Package != "aerospace" {
		t.Errorf("reqs[0].Package = %q, want %q", reqs[0].Package, "aerospace")
	}
}

func TestCollectFromPaths_predicate_no_match(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 0 {
		t.Errorf("got %d reqs, want 0", len(reqs))
	}
}

func TestCollectFromPaths_multiple_files(t *testing.T) {
	dir := t.TempDir()
	p1 := writeManifest(t, dir, "dotd-packages.yaml", `
- packages:
    - ripgrep
`)
	p2 := writeManifest(t, dir, "mac.dotd-packages.yaml", `
- when: os=macos
  packages:
    - aerospace
`)
	reqs, err := CollectFromPaths([]string{p1, p2}, map[string]string{"os": "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d reqs, want 2", len(reqs))
	}
}

func TestCollectFromPaths_complex_predicate(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "dotd-packages.yaml", `
- when: os=macos AND context=work
  packages:
    - some-work-tool
`)
	reqs, err := CollectFromPaths([]string{path}, map[string]string{"os": "macos", "context": "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("got %d reqs, want 1", len(reqs))
	}
	if reqs[0].Package != "some-work-tool" {
		t.Errorf("reqs[0].Package = %q, want %q", reqs[0].Package, "some-work-tool")
	}

	reqs, err = CollectFromPaths([]string{path}, map[string]string{"os": "macos", "context": "personal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 0 {
		t.Errorf("got %d reqs, want 0", len(reqs))
	}
}

func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
