# Unified Action System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `@action <type>` canonical annotation syntax, normalize all legacy aliases through a single conversion point, and add a `ValidateNodes` stage that catches action sequencing errors before the pipeline runs.

**Architecture:** A new `normalizeActionAnnotations` function converts `@source`, `@no-source`, `@link(dest)`, `@symlink(dest)` to canonical `{Key:"action", Args:"..."}` form immediately after scanning. `mergeActions` is simplified to handle only the canonical key. `ValidateNodes` is a new exported function inserted between `Walk` and `Filter` in both apply and check paths.

**Tech Stack:** Go 1.21+, stdlib only. No new dependencies.

**Design spec:** `docs/superpowers/specs/2026-05-13-unified-action-system-design.md`

---

## File Map

| File | Change |
|---|---|
| `internal/pipeline/actions.go` | **Create** — `normalizeActionAnnotations`, `ValidateNodes`, `validateNode` |
| `internal/pipeline/actions_test.go` | **Create** — unit tests for both functions |
| `internal/pipeline/walk.go` | **Modify** — call `normalizeActionAnnotations` after scan; simplify `mergeActions` |
| `cmd/dotd/main.go` | **Modify** — insert `ValidateNodes` in `runApply` and `runCheck` |

All test files in this repo use `package pipeline` (not `_test` suffix) — match that convention.

---

## Task 1: `normalizeActionAnnotations`

**Files:**
- Create: `internal/pipeline/actions.go`
- Create: `internal/pipeline/actions_test.go`

- [ ] **Step 1.1: Write the failing tests**

Create `internal/pipeline/actions_test.go`:

```go
package pipeline

import (
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
)

func TestNormalizeActionAnnotations(t *testing.T) {
	tests := []struct {
		name  string
		input []annotation.Annotation
		want  []annotation.Annotation
	}{
		{
			name:  "source becomes action source",
			input: []annotation.Annotation{{Key: "source", Line: 2}},
			want:  []annotation.Annotation{{Key: "action", Args: "source", Line: 2}},
		},
		{
			name:  "no-source becomes action no-source",
			input: []annotation.Annotation{{Key: "no-source", Line: 3}},
			want:  []annotation.Annotation{{Key: "action", Args: "no-source", Line: 3}},
		},
		{
			name:  "link(dest) becomes action link(dest)",
			input: []annotation.Annotation{{Key: "link", Args: "~/.gitconfig", Line: 4}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)", Line: 4}},
		},
		{
			name:  "symlink(dest) becomes action link(dest)",
			input: []annotation.Annotation{{Key: "symlink", Args: "~/.gitconfig", Line: 5}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)", Line: 5}},
		},
		{
			name:  "action passes through unchanged",
			input: []annotation.Annotation{{Key: "action", Args: "source", Line: 6}},
			want:  []annotation.Annotation{{Key: "action", Args: "source", Line: 6}},
		},
		{
			name:  "action link passes through unchanged",
			input: []annotation.Annotation{{Key: "action", Args: "link(~/dest)", Line: 7}},
			want:  []annotation.Annotation{{Key: "action", Args: "link(~/dest)", Line: 7}},
		},
		{
			name:  "when annotation passes through unchanged",
			input: []annotation.Annotation{{Key: "when", Args: "os=darwin", Line: 8}},
			want:  []annotation.Annotation{{Key: "when", Args: "os=darwin", Line: 8}},
		},
		{
			name:  "after annotation passes through unchanged",
			input: []annotation.Annotation{{Key: "after", Args: "foo", Line: 9}},
			want:  []annotation.Annotation{{Key: "after", Args: "foo", Line: 9}},
		},
		{
			name: "mixed annotations: action and non-action",
			input: []annotation.Annotation{
				{Key: "when", Args: "os=darwin", Line: 1},
				{Key: "source", Line: 2},
				{Key: "after", Args: "base", Line: 3},
			},
			want: []annotation.Annotation{
				{Key: "when", Args: "os=darwin", Line: 1},
				{Key: "action", Args: "source", Line: 2},
				{Key: "after", Args: "base", Line: 3},
			},
		},
		{
			name:  "empty slice returns empty slice",
			input: []annotation.Annotation{},
			want:  []annotation.Annotation{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeActionAnnotations(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d, len(want)=%d", len(got), len(tc.want))
			}
			for i, a := range got {
				w := tc.want[i]
				if a.Key != w.Key || a.Args != w.Args || a.Line != w.Line {
					t.Errorf("ann[%d]: got {Key:%q Args:%q Line:%d}, want {Key:%q Args:%q Line:%d}",
						i, a.Key, a.Args, a.Line, w.Key, w.Args, w.Line)
				}
			}
		})
	}
}
```

- [ ] **Step 1.2: Run tests — expect compile failure (function doesn't exist yet)**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -run TestNormalizeActionAnnotations -v 2>&1 | head -20
```

Expected: `undefined: normalizeActionAnnotations`

- [ ] **Step 1.3: Create `internal/pipeline/actions.go` with the implementation**

```go
package pipeline

import (
	"github.com/rocne/dot-dagger/internal/annotation"
)

// normalizeActionAnnotations converts legacy action annotation keys to canonical
// {Key:"action", Args:"<type>"} or {Key:"action", Args:"<type>(dest)"} form.
// Non-action annotations pass through unchanged.
func normalizeActionAnnotations(anns []annotation.Annotation) []annotation.Annotation {
	result := make([]annotation.Annotation, 0, len(anns))
	for _, a := range anns {
		switch a.Key {
		case "action":
			result = append(result, a)
		case "source":
			result = append(result, annotation.Annotation{Key: "action", Args: "source", Line: a.Line})
		case "no-source":
			result = append(result, annotation.Annotation{Key: "action", Args: "no-source", Line: a.Line})
		case "link":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		case "symlink":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		default:
			result = append(result, a)
		}
	}
	return result
}
```

- [ ] **Step 1.4: Run tests — expect pass**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -run TestNormalizeActionAnnotations -v
```

Expected: all subtests `PASS`

- [ ] **Step 1.5: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add internal/pipeline/actions.go internal/pipeline/actions_test.go
git commit -m "feat(pipeline): add normalizeActionAnnotations for canonical action form"
```

---

## Task 2: Simplify `mergeActions` to use canonical form

**Files:**
- Modify: `internal/pipeline/walk.go` (lines ~143–165 for call site; lines ~354–399 for function body)
- Modify: `internal/pipeline/actions_test.go` (add mergeActions tests)

- [ ] **Step 2.1: Write failing tests for new `mergeActions` behaviour**

Append to `internal/pipeline/actions_test.go`:

```go
func TestMergeActions_CanonicalAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		defaults []string
		anns     []annotation.Annotation // must be pre-normalized
		want     []Action
	}{
		{
			name:     "inherited source default, no annotation",
			defaults: []string{"source"},
			anns:     nil,
			want:     []Action{{Type: "source"}},
		},
		{
			name:     "action source annotation adds source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "source"}},
			want:     []Action{{Type: "source"}},
		},
		{
			name:     "action link annotation adds link",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/.gitconfig)"}},
			want:     []Action{{Type: "link", Dest: "~/.gitconfig"}},
		},
		{
			name:     "inherited source suppressed by action no-source",
			defaults: []string{"source"},
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: "no-source"}},
		},
		{
			name:     "action no-source without inherited source still adds no-source",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "action", Args: "no-source"}},
			want:     []Action{{Type: "no-source"}},
		},
		{
			name:     "action link replaces inherited link dest",
			defaults: []string{"link(~/old-dest)"},
			anns:     []annotation.Annotation{{Key: "action", Args: "link(~/new-dest)"}},
			want:     []Action{{Type: "link", Dest: "~/new-dest"}},
		},
		{
			name:     "non-action annotation ignored by mergeActions",
			defaults: nil,
			anns:     []annotation.Annotation{{Key: "when", Args: "os=darwin"}},
			want:     nil,
		},
		{
			name:     "action source and action link both applied",
			defaults: nil,
			anns: []annotation.Annotation{
				{Key: "action", Args: "source"},
				{Key: "action", Args: "link(~/.gitconfig)"},
			},
			want: []Action{{Type: "source"}, {Type: "link", Dest: "~/.gitconfig"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeActions(tc.defaults, tc.anns)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d want=%d; got=%v want=%v", len(got), len(tc.want), got, tc.want)
			}
			for i, a := range got {
				w := tc.want[i]
				if a.Type != w.Type || a.Dest != w.Dest {
					t.Errorf("action[%d]: got {Type:%q Dest:%q}, want {Type:%q Dest:%q}",
						i, a.Type, a.Dest, w.Type, w.Dest)
				}
			}
		})
	}
}
```

- [ ] **Step 2.2: Run tests — some will fail (mergeActions still uses old key names)**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -run TestMergeActions_CanonicalAnnotations -v 2>&1 | tail -20
```

Expected: failures for cases that pass `{Key:"action"}` annotations since `mergeActions` currently ignores that key.

- [ ] **Step 2.3: Update `mergeActions` in `internal/pipeline/walk.go`**

Replace the entire `mergeActions` function (currently lines ~354–399) with:

```go
// mergeActions produces the final Action list for a node.
// anns must be pre-normalized by normalizeActionAnnotations — only Key=="action" entries
// are processed; all other annotation keys are ignored.
func mergeActions(defaultActions []string, anns []annotation.Annotation) []Action {
	var actions []Action
	seen := map[string]bool{}

	// Seed with inherited defaults.
	for _, actStr := range defaultActions {
		act := parseActionString(actStr)
		if act.Type != "" && !seen[act.Type] {
			actions = append(actions, act)
			seen[act.Type] = true
		}
	}

	// Apply normalized action annotations.
	for _, a := range anns {
		if a.Key != "action" {
			continue
		}
		act := parseActionString(a.Args)
		switch act.Type {
		case "compose":
			if !seen["compose"] {
				actions = append(actions, act)
				seen["compose"] = true
			}
		case "link":
			if !seen["link"] {
				actions = append(actions, act)
				seen["link"] = true
			} else {
				for i := range actions {
					if actions[i].Type == "link" {
						actions[i].Dest = act.Dest
					}
				}
			}
		case "source":
			if !seen["source"] && !seen["no-source"] {
				actions = append(actions, act)
				seen["source"] = true
			}
		case "no-source":
			// Remove any existing source, then record no-source.
			var filtered []Action
			for _, existing := range actions {
				if existing.Type != "source" {
					filtered = append(filtered, existing)
				}
			}
			actions = filtered
			delete(seen, "source")
			if !seen["no-source"] {
				actions = append(actions, act)
				seen["no-source"] = true
			}
		}
	}

	return actions
}
```

- [ ] **Step 2.4: Call `normalizeActionAnnotations` in the file annotation path**

In `walk.go`, find the file annotation processing block (around line 146–157). It currently reads:

```go
// Parse file annotations.
anns, err := scanFileAnnotations(path)
if err != nil {
    return err
}

// Merge effective when.
fileWhen := annotation.CombineWhen(anns)
effectiveWhen := combineWhen(state.when, fileWhen)

// Compute actions: start from defaults, apply file overrides.
actions := mergeActions(state.actions, anns)
```

Add the normalization call after scanning:

```go
// Parse file annotations.
anns, err := scanFileAnnotations(path)
if err != nil {
    return err
}
anns = normalizeActionAnnotations(anns)

// Merge effective when.
fileWhen := annotation.CombineWhen(anns)
effectiveWhen := combineWhen(state.when, fileWhen)

// Compute actions: start from defaults, apply file overrides.
actions := mergeActions(state.actions, anns)
```

- [ ] **Step 2.5: Run all pipeline tests**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -v 2>&1 | tail -30
```

Expected: all tests pass including the new `TestMergeActions_CanonicalAnnotations` tests and all pre-existing Walk/Act/Order/Filter tests.

- [ ] **Step 2.6: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add internal/pipeline/walk.go internal/pipeline/actions_test.go
git commit -m "feat(pipeline): normalize action annotations before mergeActions"
```

---

## Task 3: `ValidateNodes`

**Files:**
- Modify: `internal/pipeline/actions.go` (add `ValidateNodes` and `validateNode`)
- Modify: `internal/pipeline/actions_test.go` (add ValidateNodes tests)

- [ ] **Step 3.1: Write failing tests**

Append to `internal/pipeline/actions_test.go`:

```go
func TestValidateNodes(t *testing.T) {
	// dirNode returns a RawNode that looks like a compose-target directory (no actions yet).
	dirNode := func(name string) RawNode {
		return RawNode{
			Path:          "/dotfiles/" + name,
			LogicalName:   name,
			ComposeTarget: "/dotfiles/" + name, // ComposeTarget == Path → directory
		}
	}
	// withActions clones n and sets its Actions slice.
	withActions := func(n RawNode, actions []Action) RawNode {
		n.Actions = actions
		return n
	}
	// fileNode returns a RawNode that looks like a regular file.
	fileNode := func(name string, actions []Action) RawNode {
		return RawNode{
			Path:        "/dotfiles/" + name,
			LogicalName: name,
			Actions:     actions,
			// ComposeTarget is empty → not a directory
		}
	}

	tests := []struct {
		name    string
		nodes   []RawNode
		wantErr bool
		errMsg  string
	}{
		// --- error cases ---
		{
			name:    "compose on file is an error",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: "compose"}})},
			wantErr: true,
			errMsg:  "compose is only valid on directories",
		},
		{
			name:    "link without dest is an error",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: "link", Dest: ""}})},
			wantErr: true,
			errMsg:  "link requires a destination",
		},
		{
			name: "link before compose on dir is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: "link", Dest: "~/.zshrc"},
					{Type: "compose"},
				}),
			},
			wantErr: true,
			errMsg:  "link/source must follow compose",
		},
		{
			name: "source before compose on dir is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: "source"},
					{Type: "compose"},
				}),
			},
			wantErr: true,
			errMsg:  "link/source must follow compose",
		},
		{
			name: "conflicting link destinations is an error",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: "compose"},
					{Type: "link", Dest: "~/.zshrc"},
					{Type: "link", Dest: "~/.bashrc"},
				}),
			},
			wantErr: true,
			errMsg:  "conflicting link destinations",
		},
		// --- valid cases ---
		{
			name:    "empty nodes slice is valid",
			nodes:   nil,
			wantErr: false,
		},
		{
			name:    "file with source only is valid",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: "source"}})},
			wantErr: false,
		},
		{
			name:    "file with link is valid",
			nodes:   []RawNode{fileNode("foo.sh", []Action{{Type: "link", Dest: "~/.foo"}})},
			wantErr: false,
		},
		{
			name:    "file with no actions is valid",
			nodes:   []RawNode{fileNode("foo.sh", nil)},
			wantErr: false,
		},
		{
			name: "dir with compose then link is valid",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: "compose"},
					{Type: "link", Dest: "~/.zshrc"},
				}),
			},
			wantErr: false,
		},
		{
			name: "dir with compose then source then link is valid",
			nodes: []RawNode{
				withActions(dirNode("shellrc.d"), []Action{
					{Type: "compose"},
					{Type: "source"},
					{Type: "link", Dest: "~/.zshrc"},
				}),
			},
			wantErr: false,
		},
		{
			name: "multiple errors are all reported",
			nodes: []RawNode{
				fileNode("a.sh", []Action{{Type: "compose"}}),
				fileNode("b.sh", []Action{{Type: "link", Dest: ""}}),
			},
			wantErr: true,
			errMsg:  "compose is only valid on directories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNodes(tc.nodes)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && tc.errMsg != "" {
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
			}
		})
	}
}
```

- [ ] **Step 3.2: Run tests — expect compile failure (ValidateNodes doesn't exist)**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -run TestValidateNodes -v 2>&1 | head -10
```

Expected: `undefined: ValidateNodes`

- [ ] **Step 3.3: Replace `internal/pipeline/actions.go` with the full updated file**

`actions.go` currently only imports `annotation`. The new version adds `fmt` and `strings` and appends `ValidateNodes` and `validateNode`. Replace the entire file with:

```go
package pipeline

import (
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
)

// normalizeActionAnnotations converts legacy action annotation keys to canonical
// {Key:"action", Args:"<type>"} or {Key:"action", Args:"<type>(dest)"} form.
// Non-action annotations pass through unchanged.
func normalizeActionAnnotations(anns []annotation.Annotation) []annotation.Annotation {
	result := make([]annotation.Annotation, 0, len(anns))
	for _, a := range anns {
		switch a.Key {
		case "action":
			result = append(result, a)
		case "source":
			result = append(result, annotation.Annotation{Key: "action", Args: "source", Line: a.Line})
		case "no-source":
			result = append(result, annotation.Annotation{Key: "action", Args: "no-source", Line: a.Line})
		case "link":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		case "symlink":
			result = append(result, annotation.Annotation{Key: "action", Args: "link(" + a.Args + ")", Line: a.Line})
		default:
			result = append(result, a)
		}
	}
	return result
}

// ValidateNodes checks every node for action sequencing errors. All errors are
// collected and returned together. Returns nil if all nodes are valid.
func ValidateNodes(nodes []RawNode) error {
	var errs []string
	for _, n := range nodes {
		if err := validateNode(n); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("action validation:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func validateNode(n RawNode) error {
	// Directory nodes have ComposeTarget == Path (set by Walk for composition.enabled dirs).
	isDir := n.ComposeTarget != "" && n.ComposeTarget == n.Path

	seenCompose := false
	var linkDests []string

	for _, a := range n.Actions {
		switch a.Type {
		case "compose":
			if !isDir {
				return fmt.Errorf("node %s: compose is only valid on directories", n.LogicalName)
			}
			seenCompose = true
		case "link":
			if a.Dest == "" {
				return fmt.Errorf("node %s: link requires a destination", n.LogicalName)
			}
			if isDir && !seenCompose {
				return fmt.Errorf("node %s: link/source must follow compose", n.LogicalName)
			}
			for _, prev := range linkDests {
				if prev != a.Dest {
					return fmt.Errorf("node %s: conflicting link destinations", n.LogicalName)
				}
			}
			linkDests = append(linkDests, a.Dest)
		case "source":
			if isDir && !seenCompose {
				return fmt.Errorf("node %s: link/source must follow compose", n.LogicalName)
			}
		}
	}
	return nil
}
```

- [ ] **Step 3.4: Run tests — expect pass**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -run TestValidateNodes -v
```

Expected: all subtests `PASS`

- [ ] **Step 3.5: Run full pipeline test suite**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/pipeline/ -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 3.6: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add internal/pipeline/actions.go internal/pipeline/actions_test.go
git commit -m "feat(pipeline): add ValidateNodes for action sequencing validation"
```

---

## Task 4: Wire `ValidateNodes` into the pipeline

**Files:**
- Modify: `cmd/dotd/main.go` (two call sites)

- [ ] **Step 4.1: Find the two call sites in `main.go`**

Run:
```bash
grep -n "pipeline.Walk\|pipeline.Filter" /home/rocne/git/dot-dagger/cmd/dotd/main.go
```

Expected output: two blocks — one in `runApply` (around line 299) and one in `runCheck` (around line 347). Each looks like:
```go
nodes, err := pipeline.Walk(cfg.files)
if err != nil {
    return err
}

active, err := pipeline.Filter(nodes, resolved)
```

- [ ] **Step 4.2: Insert `ValidateNodes` in both call sites**

In each block, add the validation call between `Walk` and `Filter`:

```go
nodes, err := pipeline.Walk(cfg.files)
if err != nil {
    return err
}

if err := pipeline.ValidateNodes(nodes); err != nil {
    return err
}

active, err := pipeline.Filter(nodes, resolved)
```

Make this change in BOTH occurrences (apply path and check path).

- [ ] **Step 4.3: Build**

```bash
cd /home/rocne/git/dot-dagger && go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4.4: Run full test suite**

```bash
cd /home/rocne/git/dot-dagger && go test -count=1 ./... 2>&1
```

Expected: all packages pass.

- [ ] **Step 4.5: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/main.go
git commit -m "feat(pipeline): wire ValidateNodes between Walk and Filter"
```

---

## Done

All four tasks complete. Run finishing-a-development-branch skill to verify and choose merge strategy.
