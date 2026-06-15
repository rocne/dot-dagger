# Embed Docs (agent reference) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed the authored `docs/` set into the `dotd` binary and expose `dotd docs --full` as an llms-full-style machine-readable reference for agents.

**Architecture:** A new root-level Go package (`package dotdagger`, file `embed.go`) holds a `//go:embed` of the doc dirs — the only place that can reach `docs/` (sibling of `cmd/dotd`, which can't `../docs`). A cobra-free `internal/docs.RenderProse` concatenates the embedded markdown into a blob with a leading index and full-path section headers. `cmd/dotd/docs_cmd.go` composes a provenance header + `RenderProse` + a `renderCommandRef` walk of the cobra tree, behind `dotd docs --full`. Discoverability is wired into the root command's `Long` (which `dotd --help` prints) and tested.

**Tech Stack:** Go 1.26, `embed`, `io/fs` + `testing/fstest`, `spf13/cobra`.

**Spec:** `docs/superpowers/specs/2026-06-15-embed-docs-design.md`

---

## File structure

- **Create `embed.go`** (module root, `package dotdagger`) — owns `var DocsFS embed.FS`. No logic.
- **Create `embed_test.go`** (module root) — asserts core docs embedded, `superpowers/` excluded.
- **Create `internal/docs/docs.go`** (`package docs`) — `RenderProse(fsys fs.FS) (string, error)` + helpers. Imports only stdlib; no cobra.
- **Create `internal/docs/docs_test.go`** — `fstest.MapFS`-based unit tests (order, headers, resilient ordering).
- **Create `cmd/dotd/docs_cmd.go`** (`package main`) — `newDocsCmd()` + `renderCommandRef(root *cobra.Command) string`.
- **Create `cmd/dotd/docs_cmd_test.go`** — full-render (real embed) + `renderCommandRef` + discoverability tests.
- **Modify `cmd/dotd/main.go`** — register `newDocsCmd()` in the `reference` group; add a root `Long` naming `dotd docs --full`.

---

### Task 1: Root embed package

**Files:**
- Create: `embed.go` (module root)
- Test: `embed_test.go` (module root)

- [ ] **Step 1: Write the failing test**

Create `embed_test.go`:

```go
package dotdagger

import (
	"io/fs"
	"testing"
)

func TestDocsFS_ContainsCoreDocs(t *testing.T) {
	for _, want := range []string{
		"docs/index.md",
		"docs/concepts/conditions.md",
		"docs/reference/dotd.md",
		"docs/getting-started/index.md",
	} {
		if _, err := fs.Stat(DocsFS, want); err != nil {
			t.Errorf("DocsFS missing %s: %v", want, err)
		}
	}
}

func TestDocsFS_ExcludesSuperpowers(t *testing.T) {
	if _, err := fs.Stat(DocsFS, "docs/superpowers"); err == nil {
		t.Error("DocsFS must not embed docs/superpowers (internal specs/plans)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestDocsFS -v`
Expected: FAIL — `undefined: DocsFS` (compile error; no `embed.go` yet).

- [ ] **Step 3: Write minimal implementation**

Create `embed.go`:

```go
// Package dotdagger hosts repo-root assets embedded into the binary.
//
// It exists only because //go:embed can reach files at or below the directive
// file's own directory: docs/ lives at the module root, and cmd/dotd (a
// subdirectory) cannot embed ../docs. This root package is the standard Go
// answer for embedding repo-root assets. Inclusion is by explicit pattern —
// docs/superpowers (internal specs/plans) is deliberately never listed, so it
// can never ship in the binary. Adding a new top-level doc section requires
// adding it to the patterns below.
package dotdagger

import "embed"

//go:embed docs/index.md docs/concepts docs/getting-started docs/reference
var DocsFS embed.FS
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test . -run TestDocsFS -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add embed.go embed_test.go
git commit -m "feat: embed docs/ into a root package for the binary"
```

---

### Task 2: `internal/docs.RenderProse`

**Files:**
- Create: `internal/docs/docs.go`
- Test: `internal/docs/docs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/docs/docs_test.go`:

```go
package docs

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestRenderProse_OrderHeadersBodies(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":                 {Data: []byte("INTRO")},
		"docs/concepts/conditions.md":   {Data: []byte("COND")},
		"docs/reference/dotd.md":        {Data: []byte("REF")},
		"docs/getting-started/index.md": {Data: []byte("START")},
	}
	out, err := RenderProse(fsys)
	if err != nil {
		t.Fatal(err)
	}

	for _, h := range []string{
		"# === docs/index.md ===",
		"# === docs/getting-started/index.md ===",
		"# === docs/concepts/conditions.md ===",
		"# === docs/reference/dotd.md ===",
	} {
		if !strings.Contains(out, h) {
			t.Errorf("missing section header %q", h)
		}
	}

	// Exact curated order: index -> getting-started -> concepts -> reference.
	order := []string{
		"# === docs/index.md ===",
		"# === docs/getting-started/index.md ===",
		"# === docs/concepts/conditions.md ===",
		"# === docs/reference/dotd.md ===",
	}
	last := -1
	for _, h := range order {
		i := strings.Index(out, h)
		if i <= last {
			t.Errorf("section %q out of order (idx %d, prev %d)", h, i, last)
		}
		last = i
	}

	if !strings.Contains(out, "COND") {
		t.Error("missing doc body content")
	}
}

func TestRenderProse_UnknownDirsAppendedAlphabetically(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":         {Data: []byte("I")},
		"docs/reference/a.md":   {Data: []byte("R")},
		"docs/guides/intro.md":  {Data: []byte("G")},
		"docs/zzz/last.md":      {Data: []byte("Z")},
	}
	out, err := RenderProse(fsys)
	if err != nil {
		t.Fatal(err)
	}
	ri := strings.Index(out, "docs/reference/a.md")
	gi := strings.Index(out, "docs/guides/intro.md")
	zi := strings.Index(out, "docs/zzz/last.md")
	// Known (reference) before unknown; unknown dirs alphabetical (guides<zzz).
	if !(ri >= 0 && ri < gi && gi < zi) {
		t.Errorf("bad order: reference=%d guides=%d zzz=%d\n%s", ri, gi, zi, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/docs/ -v`
Expected: FAIL — `undefined: RenderProse` (no `docs.go` yet).

- [ ] **Step 3: Write minimal implementation**

Create `internal/docs/docs.go`:

```go
// Package docs assembles the embedded documentation into a single text blob.
// It is pure: it reads an fs.FS and returns a string, with no cobra or other
// command-layer dependency, so it is unit-testable against a fstest.MapFS.
package docs

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// docsRoot is the top-level directory the embedded FS is rooted under
// (//go:embed keeps the "docs/" path prefix).
const docsRoot = "docs"

// priority is the curated render order of top-level entries under docsRoot.
// Entries not listed here are appended after these, alphabetically — so a new
// doc section still ships (just unordered) rather than silently disappearing.
var priority = []string{"index.md", "getting-started", "concepts", "reference"}

// RenderProse concatenates the embedded markdown under docsRoot into one blob:
// a leading index of the sections that follow, then each file body prefixed
// with a "# === <repo-relative-path> ===" separator. The full path in the
// separator is load-bearing: the docs contain relative cross-links such as
// [x](../concepts/conditions.md) which don't resolve on stdout, but an agent
// can trace such a link to the matching section header in the same blob.
func RenderProse(fsys fs.FS) (string, error) {
	files, err := orderedFiles(fsys)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("# Documentation index\n\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteByte('\n')

	for _, f := range files {
		body, err := fs.ReadFile(fsys, f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "# === %s ===\n\n", f)
		b.Write(body)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// orderedFiles returns the .md files under docsRoot in curated-then-alphabetical
// order (see priority). Files within a directory are alphabetical.
func orderedFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, docsRoot)
	if err != nil {
		return nil, err
	}

	rank := make(map[string]int, len(priority))
	for i, name := range priority {
		rank[name] = i
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.SliceStable(names, func(i, j int) bool {
		ri, oki := rank[names[i]]
		rj, okj := rank[names[j]]
		switch {
		case oki && okj:
			return ri < rj
		case oki:
			return true
		case okj:
			return false
		default:
			return names[i] < names[j]
		}
	})

	var files []string
	for _, name := range names {
		full := path.Join(docsRoot, name)
		info, err := fs.Stat(fsys, full)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			subs, err := mdFiles(fsys, full)
			if err != nil {
				return nil, err
			}
			files = append(files, subs...)
		} else if strings.HasSuffix(name, ".md") {
			files = append(files, full)
		}
	}
	return files, nil
}

// mdFiles walks dir and returns all .md files, sorted.
func mdFiles(fsys fs.FS, dir string) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/docs/ -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/docs/docs.go internal/docs/docs_test.go
git commit -m "feat: add docs.RenderProse to assemble the embedded blob"
```

---

### Task 3: `renderCommandRef`

**Files:**
- Create: `cmd/dotd/docs_cmd.go` (renderCommandRef only this task)
- Test: `cmd/dotd/docs_cmd_test.go` (renderCommandRef test this task)

- [ ] **Step 1: Write the failing test**

Create `cmd/dotd/docs_cmd_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRenderCommandRef_IncludesVisibleSkipsHelpers(t *testing.T) {
	root := &cobra.Command{Use: "dotd"}
	root.AddCommand(&cobra.Command{Use: "apply", Short: "apply things", Long: "Apply the dotfiles."})
	root.AddCommand(&cobra.Command{Use: "secret", Short: "x", Hidden: true})
	root.AddCommand(&cobra.Command{Use: "completion", Short: "gen completion"})

	out := renderCommandRef(root)

	if !strings.Contains(out, "dotd apply") {
		t.Error("CLI reference missing 'dotd apply'")
	}
	if !strings.Contains(out, "Apply the dotfiles.") {
		t.Error("CLI reference missing apply Long text")
	}
	if strings.Contains(out, "secret") {
		t.Error("hidden command leaked into CLI reference")
	}
	if strings.Contains(out, "dotd completion") {
		t.Error("completion command leaked into CLI reference")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotd/ -run TestRenderCommandRef -v`
Expected: FAIL — `undefined: renderCommandRef`.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/dotd/docs_cmd.go`:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// renderCommandRef walks the command tree under root and renders each command's
// help (usage line, Long or Short, then its LOCAL flags) into a single CLI
// Reference section. Hidden commands and the built-in help/completion commands
// are skipped so the section stays signal. Global/persistent flags are
// deliberately NOT repeated per command — they are inherited by every command
// and are documented once via `dotd --help` and docs/reference/dotd.md;
// repeating the 8-flag block ~20 times would bloat the agent-facing blob for no
// gain. Including the full CLI surface here means an agent gets everything in a
// single call instead of discovering and chaining N `dotd <cmd> --help` calls.
func renderCommandRef(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString("# === CLI Reference ===\n\n")
	b.WriteString("Global flags apply to every command and are documented once " +
		"under `dotd --help` and docs/reference/dotd.md; they are omitted per-command below.\n\n")

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n", sub.CommandPath())
			fmt.Fprintf(&b, "Usage: %s\n\n", sub.UseLine())
			if sub.Long != "" {
				b.WriteString(sub.Long)
			} else {
				b.WriteString(sub.Short)
			}
			b.WriteString("\n\n")
			// LocalFlags() excludes flags inherited from the root (the globals),
			// so this shows only the command's own flags (e.g. docs's --full).
			if usages := sub.LocalFlags().FlagUsages(); strings.TrimSpace(usages) != "" {
				b.WriteString("Flags:\n")
				b.WriteString(usages)
				b.WriteByte('\n')
			}
			walk(sub)
		}
	}
	walk(root)
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotd/ -run TestRenderCommandRef -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/dotd/docs_cmd.go cmd/dotd/docs_cmd_test.go
git commit -m "feat: add renderCommandRef to dump the cobra command tree"
```

---

### Task 4: `dotd docs --full` command + registration + discoverability

**Files:**
- Modify: `cmd/dotd/docs_cmd.go` (add `newDocsCmd`)
- Modify: `cmd/dotd/docs_cmd_test.go` (add full-render + discoverability tests)
- Modify: `cmd/dotd/main.go:146-152` (add root `Long`) and `cmd/dotd/main.go:272-277` (register command)

- [ ] **Step 1: Write the failing tests**

Append to `cmd/dotd/docs_cmd_test.go` (add `"bytes"` to its imports):

```go
func TestDocsCmd_FullRendersEmbeddedReference(t *testing.T) {
	cmd := newDocsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("full", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "embedded reference") {
		t.Error("missing provenance header")
	}
	if !strings.Contains(s, "# === docs/concepts/") {
		t.Error("missing embedded concepts section")
	}
	if !strings.Contains(s, "CLI Reference") {
		t.Error("missing CLI reference section")
	}
}

func TestRootHelp_MentionsDocsFull(t *testing.T) {
	// `--help` short-circuits in cobra before PersistentPreRunE, so this does
	// not run resolvePaths. run() is the existing helper (newRootCmd + buffered
	// out/err + Execute) defined in main_test.go.
	out, err := run(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "docs --full") {
		t.Errorf("`dotd --help` does not mention 'docs --full':\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/dotd/ -run 'TestDocsCmd_FullRendersEmbeddedReference|TestRootHelp_MentionsDocsFull' -v`
Expected: FAIL — `undefined: newDocsCmd` and root help lacks `docs --full`.

- [ ] **Step 3a: Add `newDocsCmd` to `cmd/dotd/docs_cmd.go`**

Update the import block and append the constructor. The file's imports become:

```go
import (
	"fmt"
	"strings"

	dotdagger "github.com/rocne/dot-dagger"
	"github.com/rocne/dot-dagger/internal/docs"
	"github.com/spf13/cobra"
)
```

Append:

```go
// newDocsCmd builds the `docs` command. With --full it prints the complete
// machine-readable reference (an llms-full-style blob): a provenance header,
// the embedded prose, then the full CLI reference. Without --full it falls
// through to cobra's own help (which lists --full); per-topic human output is
// a future follow-up that reuses RenderProse.
func newDocsCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Print embedded documentation",
		Long: `Print dot-dagger's documentation, embedded in the binary.

With --full, prints the complete machine-readable reference to stdout: every
concept and reference page plus the full CLI reference, as one llms-full-style
blob — the form intended for agents and tooling. Offline; no doc-site needed.

Examples:
  dotd docs --full              # complete reference (for agents)
  dotd docs --full | less       # page it
  dotd docs --full > dotd.txt   # capture to a file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !full {
				return cmd.Help()
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# dotd %s — embedded reference\n\n", version)
			prose, err := docs.RenderProse(dotdagger.DocsFS)
			if err != nil {
				return fmt.Errorf("render docs: %w", err)
			}
			fmt.Fprint(out, prose)
			fmt.Fprint(out, renderCommandRef(cmd.Root()))
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "print the complete machine-readable reference (for agents)")
	return cmd
}
```

Note: `version` is the package-level build var in `cmd/dotd/main.go:30` (defaults `"dev"`); same package, no import needed. `GroupID` is intentionally not set in the constructor — it's assigned at registration in Step 3c, matching the pattern used by the other command constructors.

- [ ] **Step 3b: Add the root `Long` in `cmd/dotd/main.go`**

In `newRootCmd`, the root command literal at `cmd/dotd/main.go:146` currently has no `Long`. Add one so the custom help func (which prints `Long`) advertises the command:

```go
	root := &cobra.Command{
		Use:   ecosystem.ToolD,
		Short: "Dotfiles manager — env resolution, DAG, symlinks, and init.sh generation",
		Long: `dotd — dotfiles manager: env resolution, DAG, symlinks, and init.sh generation.

Run 'dotd docs --full' for the complete machine-readable reference (concepts,
reference docs, and the full CLI help) embedded in the binary — intended for
agents and tooling.`,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
```

- [ ] **Step 3c: Register the command in `cmd/dotd/main.go`**

In the reference-group block at `cmd/dotd/main.go:272-277`, add `docsCmd`:

```go
	conceptsCmd := newConceptsCmd()
	conceptsCmd.GroupID = "reference"
	docsCmd := newDocsCmd()
	docsCmd.GroupID = "reference"
	completionCmd := newCompletionCmd()
	completionCmd.GroupID = "reference"
	root.AddCommand(completionCmd)
	root.AddCommand(conceptsCmd)
	root.AddCommand(docsCmd)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/dotd/ -run 'TestDocsCmd_FullRendersEmbeddedReference|TestRootHelp_MentionsDocsFull|TestRenderCommandRef' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add cmd/dotd/docs_cmd.go cmd/dotd/docs_cmd_test.go cmd/dotd/main.go
git commit -m "feat: add 'dotd docs --full' embedded agent reference"
```

---

### Task 5: Full validation

- [ ] **Step 1: Format, build, and run the whole test suite**

Run: `gofmt -w . && go build ./... && go test ./...`
Expected: `gofmt` rewrites any hand-aligned literals (e.g. the `fstest.MapFS`
maps) with no remaining diff on re-run; build succeeds; all packages PASS.

- [ ] **Step 2: Smoke-test the real binary**

Run:
```bash
go run ./cmd/dotd docs --full | head -40
go run ./cmd/dotd docs --full | grep -c '# === '   # >0 section headers
go run ./cmd/dotd --help | grep 'docs --full'       # discoverability
go run ./cmd/dotd docs --full | grep -c 'superpowers'  # expect 0
```
Expected: blob prints with provenance header + section headers + a CLI Reference; `--help` shows `docs --full`; zero `superpowers` matches.

- [ ] **Step 3: Update TODO**

In `.claude/TODO.md`, move the "Embed all documentation into the binary" item from 🟢 EXPLORE to ✅ DONE (note `dotd docs --full`, spec/plan dated 2026-06-15). Leave the human-topic-view / man-page / `concepts` dedup notes as follow-ups. (Local file — do not commit.)

---

## Self-review notes

- **Spec coverage:** root embed pkg (Task 1) ✓; explicit-include exclusion of `superpowers/` (Task 1 test) ✓; cobra-free `RenderProse` + resilient ordering + full-path headers + leading index (Task 2) ✓; `renderCommandRef` skipping help/completion/hidden, local-flags-only to avoid repeating globals ~20× (Task 3) ✓; `--full` flag + provenance header + bare-`docs`→help (Task 4) ✓; discoverability in root `--help` + test (Task 4) ✓; tests for path-headers, exact order, no-superpowers, version header, discoverability ✓.
- **Cross-link traceability** is satisfied structurally: separators use full repo-relative paths matching link targets; no separate transform (per spec, ship raw).
- **Out of scope (unchanged):** human `dotd docs <topic>`/`--list`, man pages, `concepts` dedup (coexistence is correct — digest vs full pages).
