# v2 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the annotation scanner, introduce the `.dagger` config file format, and define the v2 node type hierarchy with logical name derivation.

**Architecture:** Three packages — `internal/annotation/` (scanner, rewritten for new `@key(args)` syntax), `internal/dagger/` (new `.dagger` file parser replacing `internal/daggeryaml/`), `internal/node/` (logical name derivation). Each is independently testable. Downstream packages (walk, fileset, pipeline) are broken until Plan 3 — that's expected for a v2 rewrite.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, standard library only.

---

## Context

This is the foundation for a ground-up v2 redesign. Key changes from v1:

| v1 | v2 |
|----|-----|
| `# @when os=macos` | `# @when(os=macos)` |
| `# @symlink ~/.tmux.conf` | `# @link(~/.tmux.conf)` |
| Blank line stops annotation scan | Blank lines allowed in annotation block |
| `.dotd.yaml` per-dir config | `.dagger` per-dir config |
| Sections: `dotd:`, `link:`, `env:` | Flat structure: when/link_root/actions/defaults/files/composition |
| `files:` is a list with `path:` inside | `files:` is a dict, path is the key |

The spec is at `docs/superpowers/specs/2026-05-07-v2-redesign-design.md`.

---

## File Map

| File | Status | Responsibility |
|------|--------|----------------|
| `internal/annotation/annotation.go` | Rewrite | Scanner for new `@key(args)` syntax; `Get`/`First`/`CombineWhen` helpers |
| `internal/annotation/annotation_test.go` | Rewrite | Full test suite for new syntax and scanner rules |
| `internal/dagger/dagger.go` | Create | Parse `.dagger` files; `BasicNode`/`NamedNode`/`ComposableNode` YAML types |
| `internal/dagger/dagger_test.go` | Create | Parser tests |
| `internal/node/node.go` | Create | `DeriveName` — logical name derivation from relative path |
| `internal/node/node_test.go` | Create | Derivation tests |
| `internal/daggeryaml/` | Delete (after Plan 3 migrates callers) | Superseded by `internal/dagger/` |

---

## Task 1: Rewrite Annotation Scanner

**Files:**
- Rewrite: `internal/annotation/annotation.go`
- Rewrite: `internal/annotation/annotation_test.go`

### New syntax rules

`@key` — zero-arg annotation (e.g., `@source`, `@no-source`, `@disable`)
`@key(args)` — annotation with args (e.g., `@when(os=macos)`, `@link(~/.tmux.conf)`)

### Scanner rules (v2)

1. If first line is a shebang (`#!`), skip it
2. Read comment lines (`#` or `//`)
3. **First non-comment, non-blank line stops the scan**
4. Non-`@` comment lines are ignored without stopping the scan
5. Blank lines do NOT stop the scan (v1 difference: blank lines did stop it)

- [ ] **Step 1.1: Write failing tests**

Replace `internal/annotation/annotation_test.go` entirely:

```go
package annotation

import (
	"strings"
	"testing"
)

func TestScan_HashComment(t *testing.T) {
	anns, err := Scan(strings.NewReader("# @when(os=macos)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" || anns[0].Args != "os=macos" || anns[0].Line != 1 {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_SlashComment(t *testing.T) {
	anns, err := Scan(strings.NewReader("// @name(my.name)"))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "name" || anns[0].Args != "my.name" {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_ZeroArgAnnotations(t *testing.T) {
	input := "# @source\n# @no-source\n# @disable"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 3 {
		t.Fatalf("want 3 annotations, got %d: %+v", len(anns), anns)
	}
	if anns[0].Key != "source" || anns[0].Args != "" {
		t.Errorf("anns[0] = %+v", anns[0])
	}
	if anns[1].Key != "no-source" || anns[1].Args != "" {
		t.Errorf("anns[1] = %+v", anns[1])
	}
	if anns[2].Key != "disable" || anns[2].Args != "" {
		t.Errorf("anns[2] = %+v", anns[2])
	}
}

func TestScan_ShebangSkipped(t *testing.T) {
	input := "#!/bin/bash\n# @when(os=macos)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" || anns[0].Line != 2 {
		t.Errorf("got %+v", anns)
	}
}

func TestScan_BlankLinePermitted(t *testing.T) {
	input := "# @when(os=macos)\n\n# @after(shellrc.base)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 2 {
		t.Fatalf("blank line should not stop scan, want 2 annotations, got %d: %+v", len(anns), anns)
	}
}

func TestScan_NonCommentStops(t *testing.T) {
	input := "# @when(os=macos)\nexport FOO=bar\n# @after(shellrc.base)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 {
		t.Errorf("non-comment line should stop scan, want 1 annotation, got %d: %+v", len(anns), anns)
	}
}

func TestScan_NonAtCommentIgnored(t *testing.T) {
	input := "# just a comment\n# @when(os=macos)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "when" {
		t.Errorf("non-@ comment should be ignored, got %+v", anns)
	}
}

func TestScan_MultipleAnnotations(t *testing.T) {
	input := "# @when(os=macos)\n# @after(shellrc.base)\n# @link(~/.tmux.conf)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Annotation{
		{Key: "when", Args: "os=macos", Line: 1},
		{Key: "after", Args: "shellrc.base", Line: 2},
		{Key: "link", Args: "~/.tmux.conf", Line: 3},
	}
	if len(anns) != len(want) {
		t.Fatalf("want %d, got %d: %+v", len(want), len(anns), anns)
	}
	for i := range want {
		if anns[i] != want[i] {
			t.Errorf("anns[%d] = %+v, want %+v", i, anns[i], want[i])
		}
	}
}

func TestScan_EmptyInput(t *testing.T) {
	anns, err := Scan(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 0 {
		t.Errorf("want 0 annotations, got %+v", anns)
	}
}

func TestScan_PathPrefix(t *testing.T) {
	// @after accepts path prefixes ending in /
	input := "# @after(shellrc/)"
	anns, err := Scan(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Key != "after" || anns[0].Args != "shellrc/" {
		t.Errorf("got %+v", anns)
	}
}

func TestGet(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Args: "os=macos"},
		{Key: "name", Args: "foo"},
		{Key: "when", Args: "context=work"},
	}
	got := Get(anns, "when")
	if len(got) != 2 {
		t.Fatalf("Get() len = %d, want 2", len(got))
	}
	if got[0].Args != "os=macos" || got[1].Args != "context=work" {
		t.Errorf("Get() = %+v, unexpected values", got)
	}
}

func TestFirst(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Args: "os=macos"},
		{Key: "name", Args: "foo"},
	}
	got, ok := First(anns, "name")
	if !ok {
		t.Fatal("First() ok = false, want true")
	}
	if got.Args != "foo" {
		t.Errorf("First().Args = %q, want %q", got.Args, "foo")
	}
	_, ok = First(anns, "after")
	if ok {
		t.Error("First() ok = true for missing key, want false")
	}
}

func TestCombineWhen(t *testing.T) {
	tests := []struct {
		name string
		anns []Annotation
		want string
	}{
		{
			name: "single when",
			anns: []Annotation{{Key: "when", Args: "os=macos"}},
			want: "(os=macos)",
		},
		{
			name: "multiple when",
			anns: []Annotation{
				{Key: "when", Args: "os=macos OR os=linux"},
				{Key: "when", Args: "context=work"},
			},
			want: "(os=macos OR os=linux) AND (context=work)",
		},
		{
			name: "no when",
			anns: []Annotation{{Key: "name", Args: "foo"}},
			want: "",
		},
		{
			name: "empty when args ignored",
			anns: []Annotation{{Key: "when", Args: ""}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineWhen(tt.anns)
			if got != tt.want {
				t.Errorf("CombineWhen() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 1.2: Run tests to verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/annotation/... -v 2>&1 | head -40
```

Expected: compilation error — `Annotation` struct has no `Args` field, `Scan` signature mismatch.

- [ ] **Step 1.3: Rewrite `internal/annotation/annotation.go`**

```go
// Package annotation scans files for @key and @key(args) annotations
// in comment lines at the top of a file.
package annotation

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Annotation is a single @key or @key(args) found in a comment line.
type Annotation struct {
	Key  string
	Args string // content inside parens; empty for zero-arg annotations
	Line int
}

// Scan reads r and returns all @key/(args) annotations found in the header block.
//
// Scanner rules:
//  1. If the first line is a shebang (#!), skip it.
//  2. Read comment lines (# or //).
//  3. The first non-comment, non-blank line stops the scan.
//  4. Non-@ comment lines are ignored without stopping the scan.
//  5. Blank lines do not stop the scan.
func Scan(r io.Reader) ([]Annotation, error) {
	var anns []Annotation
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Skip shebang on line 1.
		if lineNum == 1 && strings.HasPrefix(line, "#!") {
			continue
		}

		// Blank lines are allowed in the annotation block.
		if line == "" {
			continue
		}

		var content string
		if strings.HasPrefix(line, "//") {
			content = strings.TrimSpace(line[2:])
		} else if strings.HasPrefix(line, "#") {
			content = strings.TrimSpace(line[1:])
		} else {
			// Non-comment, non-blank line: stop scanning.
			break
		}

		if content == "" || !strings.HasPrefix(content, "@") {
			continue // non-@ comment: ignore, keep scanning
		}

		rest := content[1:] // strip leading @

		// Parse @key or @key(args)
		key, args := parseKeyArgs(rest)
		if key == "" {
			continue
		}
		anns = append(anns, Annotation{Key: key, Args: args, Line: lineNum})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("annotation: scan: %w", err)
	}
	return anns, nil
}

// parseKeyArgs splits "key(args)" into key and args.
// For "key" with no parens, args is "".
// For "key()" args is "".
func parseKeyArgs(s string) (key, args string) {
	i := strings.IndexByte(s, '(')
	if i < 0 {
		// No parens — zero-arg annotation.
		return strings.TrimSpace(s), ""
	}
	key = strings.TrimSpace(s[:i])
	rest := s[i+1:]
	j := strings.LastIndexByte(rest, ')')
	if j < 0 {
		// Malformed — treat whole thing as key, no args.
		return strings.TrimSpace(s), ""
	}
	args = strings.TrimSpace(rest[:j])
	return key, args
}

// Get returns all annotations with the given key.
func Get(anns []Annotation, key string) []Annotation {
	var result []Annotation
	for _, a := range anns {
		if a.Key == key {
			result = append(result, a)
		}
	}
	return result
}

// First returns the first annotation with the given key and true,
// or the zero value and false if none is found.
func First(anns []Annotation, key string) (Annotation, bool) {
	for _, a := range anns {
		if a.Key == key {
			return a, true
		}
	}
	return Annotation{}, false
}

// CombineWhen returns a combined @when expression from all @when annotations,
// joining them with AND. Each expression is wrapped in parentheses.
// Returns an empty string if no @when annotations are present.
func CombineWhen(anns []Annotation) string {
	var parts []string
	for _, a := range anns {
		if a.Key == "when" && a.Args != "" {
			parts = append(parts, "("+a.Args+")")
		}
	}
	return strings.Join(parts, " AND ")
}
```

- [ ] **Step 1.4: Run tests to verify they pass**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/annotation/... -v
```

Expected: all tests PASS.

- [ ] **Step 1.5: Commit**

```bash
git add internal/annotation/annotation.go internal/annotation/annotation_test.go
git commit -m "feat(annotation): rewrite scanner for v2 @key(args) syntax"
```

---

## Task 2: Create `.dagger` File Parser

**Files:**
- Create: `internal/dagger/dagger.go`
- Create: `internal/dagger/dagger_test.go`

The `.dagger` file is a YAML file at the root of any directory. It replaces `.dotd.yaml`. The top-level structure is a `ComposableNode`.

- [ ] **Step 2.1: Write failing tests**

Create `internal/dagger/dagger_test.go`:

```go
package dagger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_Empty(t *testing.T) {
	node, err := Load(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "" || node.LinkRoot != "" || len(node.Actions) != 0 {
		t.Errorf("empty input should produce zero-value node, got %+v", node)
	}
}

func TestLoad_BasicFields(t *testing.T) {
	input := `when: os=macos
link_root: ~/relative
actions:
  - source
  - link(~/.tmux.conf)
name: my.name`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "os=macos" {
		t.Errorf("When = %q, want %q", node.When, "os=macos")
	}
	if node.LinkRoot != "~/relative" {
		t.Errorf("LinkRoot = %q, want %q", node.LinkRoot, "~/relative")
	}
	if len(node.Actions) != 2 || node.Actions[0] != "source" || node.Actions[1] != "link(~/.tmux.conf)" {
		t.Errorf("Actions = %v", node.Actions)
	}
	if node.Name != "my.name" {
		t.Errorf("Name = %q, want %q", node.Name, "my.name")
	}
}

func TestLoad_Defaults(t *testing.T) {
	input := `defaults:
  when: context=work
  link_root: ~/apps
  actions:
    - no-source`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if node.Defaults.When != "context=work" {
		t.Errorf("Defaults.When = %q, want %q", node.Defaults.When, "context=work")
	}
	if node.Defaults.LinkRoot != "~/apps" {
		t.Errorf("Defaults.LinkRoot = %q", node.Defaults.LinkRoot)
	}
	if len(node.Defaults.Actions) != 1 || node.Defaults.Actions[0] != "no-source" {
		t.Errorf("Defaults.Actions = %v", node.Defaults.Actions)
	}
}

func TestLoad_Composition(t *testing.T) {
	input := `composition:
  enabled: true`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !node.Composition.Enabled {
		t.Error("Composition.Enabled = false, want true")
	}
}

func TestLoad_Files(t *testing.T) {
	input := `files:
  settings.json:
    when: os=macos
    name: nvim.settings
    actions:
      - link(settings.json)
  dot-gitconfig-work:
    when: context=work
    actions:
      - link(~/.gitconfig)`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(node.Files))
	}
	s, ok := node.Files["settings.json"]
	if !ok {
		t.Fatal("Files[settings.json] not found")
	}
	if s.When != "os=macos" || s.Name != "nvim.settings" {
		t.Errorf("settings.json = %+v", s)
	}
	if len(s.Actions) != 1 || s.Actions[0] != "link(settings.json)" {
		t.Errorf("settings.json.Actions = %v", s.Actions)
	}
	g, ok := node.Files["dot-gitconfig-work"]
	if !ok {
		t.Fatal("Files[dot-gitconfig-work] not found")
	}
	if g.When != "context=work" {
		t.Errorf("dot-gitconfig-work.When = %q", g.When)
	}
}

func TestLoadFile_NotExist(t *testing.T) {
	node, err := LoadFile(filepath.Join(t.TempDir(), ".dagger"))
	if err != nil {
		t.Fatalf("LoadFile non-existent: %v", err)
	}
	if node.When != "" || node.Composition.Enabled {
		t.Errorf("non-existent file should return zero value, got %+v", node)
	}
}

func TestLoadFile_RealFile(t *testing.T) {
	dir := t.TempDir()
	content := "when: os=macos\ncomposition:\n  enabled: true\n"
	if err := os.WriteFile(filepath.Join(dir, ".dagger"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	node, err := LoadFile(filepath.Join(dir, ".dagger"))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "os=macos" || !node.Composition.Enabled {
		t.Errorf("got %+v", node)
	}
}
```

- [ ] **Step 2.2: Run tests to verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/dagger/... -v 2>&1 | head -20
```

Expected: compilation error — package `dagger` does not exist.

- [ ] **Step 2.3: Create `internal/dagger/dagger.go`**

```go
// Package dagger loads and parses .dagger per-directory config files.
package dagger

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// BasicNode is the base metadata that can appear on any node.
type BasicNode struct {
	When     string   `yaml:"when"`
	LinkRoot string   `yaml:"link_root"`
	Actions  []string `yaml:"actions"`
}

// NamedNode extends BasicNode with an optional logical name override.
// Used for entries in the files: dict.
type NamedNode struct {
	BasicNode `yaml:",inline"`
	Name      string `yaml:"name"`
}

// CompositionConfig controls whether this directory is a compose target.
type CompositionConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ComposableNode is the top-level structure of a .dagger file.
// It represents a directory node with all possible fields.
type ComposableNode struct {
	NamedNode   `yaml:",inline"`
	Defaults    BasicNode            `yaml:"defaults"`
	Files       map[string]NamedNode `yaml:"files"`
	Composition CompositionConfig    `yaml:"composition"`
}

// Load parses a .dagger file from r.
// An empty or missing file is valid and returns a zero-value ComposableNode.
func Load(r io.Reader) (*ComposableNode, error) {
	var node ComposableNode
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&node); err != nil && err != io.EOF {
		return nil, fmt.Errorf("dagger: decode: %w", err)
	}
	return &node, nil
}

// LoadFile reads a .dagger file at path.
// If the file does not exist, returns a zero-value ComposableNode without error.
func LoadFile(path string) (*ComposableNode, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &ComposableNode{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dagger: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}
```

- [ ] **Step 2.4: Run tests to verify they pass**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/dagger/... -v
```

Expected: all tests PASS.

- [ ] **Step 2.5: Commit**

```bash
git add internal/dagger/
git commit -m "feat(dagger): add .dagger file parser with v2 node schema"
```

---

## Task 3: Logical Name Derivation

**Files:**
- Create: `internal/node/node.go`
- Create: `internal/node/node_test.go`

`DeriveName` computes the dot-separated logical name from a path relative to the dotfiles repo root. Applied per component: strip `nosync-`, strip `dot-`, strip file extension from final component only.

- [ ] **Step 3.1: Write failing tests**

Create `internal/node/node_test.go`:

```go
package node

import "testing"

func TestDeriveName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"shellrc/helpers.sh", "shellrc.helpers"},
		{"shellrc/math.sh", "shellrc.math"},
		{"tmux/shellrc/helpers.sh", "tmux.shellrc.helpers"},
		{"nosync-work/shellrc/aliases.sh", "work.shellrc.aliases"},
		{"conf/dot-tmux.conf", "conf.tmux.conf"},
		{"nosync-dot-secrets/api.sh", "secrets.api"},
		{"conf/dot-config/tmux/tmux.conf", "conf.config.tmux.tmux"},
		// dot- stripped at every path level
		{"dot-foo/shellrc/bar.sh", "foo.shellrc.bar"},
		// nosync- stripped before dot-
		{"nosync-dot-work/shellrc/bar.sh", "work.shellrc.bar"},
		// extension stripped from final component only
		{"shellrc/dot-aliases.sh", "shellrc.aliases"},
		// no extension — no change to final component
		{"bin/my-tool", "bin.my-tool"},
		// single component
		{"aliases.sh", "aliases"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := DeriveName(c.path)
			if got != c.want {
				t.Errorf("DeriveName(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/node/... -v 2>&1 | head -20
```

Expected: compilation error — package `node` does not exist.

- [ ] **Step 3.3: Create `internal/node/node.go`**

```go
// Package node provides types and utilities for dotfiles nodes.
package node

import (
	"path/filepath"
	"strings"
)

// DeriveName computes the dot-separated logical name from a path
// relative to the dotfiles repo root.
//
// Per path component:
//  1. Strip leading "nosync-"
//  2. Strip leading "dot-" (after nosync-)
//  3. Strip file extension from the final component only
func DeriveName(relPath string) string {
	components := strings.Split(filepath.ToSlash(relPath), "/")
	result := make([]string, len(components))
	for i, c := range components {
		c = strings.TrimPrefix(c, "nosync-")
		c = strings.TrimPrefix(c, "dot-")
		if i == len(components)-1 {
			if ext := filepath.Ext(c); ext != "" {
				c = strings.TrimSuffix(c, ext)
			}
		}
		result[i] = c
	}
	return strings.Join(result, ".")
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/node/... -v
```

Expected: all tests PASS.

- [ ] **Step 3.5: Run full test suite to check for regressions**

```bash
cd /home/rocne/git/dot-dagger && go build ./... && go test ./internal/annotation/... ./internal/dagger/... ./internal/node/... -v
```

Expected: all three packages pass. Other packages that still reference old annotation API may emit compilation errors — that's expected and will be resolved in Plan 3.

- [ ] **Step 3.6: Commit**

```bash
git add internal/node/
git commit -m "feat(node): add logical name derivation"
```

---

## Plan Scope

This plan covers Plan 1 of 4. Remaining plans:

- **Plan 2 — Env & Config:** `env.yaml` with `$(...)` shell interpolation, `config.yaml`, env resolution order, `dotd get-os`/`dotd get-hostname` hidden commands, `dotd env *` / `dotd config *` CLI subcommands
- **Plan 3 — Pipeline:** rewrite walk/fileset/dag/linker/initgen/composer to use new node types and `.dagger` format; `dotd apply` / `dotd check`
- **Plan 4 — CLI & Init:** `dotd init` interactive wizard, `dotd list`, `dotd bundle`, `dotd help --all`
