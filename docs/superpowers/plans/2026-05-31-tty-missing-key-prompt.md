# TTY-Aware Missing Key Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `dotd` is run in an interactive terminal and required env keys are missing, prompt for them instead of halting — using the values for the current run and printing a YAML hint to persist them.

**Architecture:** Add `pipeline.CollectMissingKeys` (AST-based key scan, no evaluation) alongside the existing `pipeline.Filter`. Add `filterWithPrompt` in `cmd/dotd` that wraps both: on TTY with missing keys it prompts via `huh`, augments the resolved env, then calls `Filter`. All `pipeline.Filter` callsites in command files are replaced with `filterWithPrompt`.

**Tech Stack:** Go, `github.com/charmbracelet/huh` (forms), `github.com/charmbracelet/x/term` (TTY detection) — both already in `go.mod`.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/pipeline/filter.go` | Modify | Add `CollectMissingKeys` |
| `internal/pipeline/filter_test.go` | Modify | Tests for `CollectMissingKeys` |
| `cmd/dotd/filter_prompt.go` | Create | `filterWithPrompt`, `promptMissingKeys`, `printPersistHint` |
| `cmd/dotd/filter_prompt_test.go` | Create | Tests for non-TTY path and zero-missing-keys path |
| `cmd/dotd/main.go` | Modify | Replace `pipeline.Filter` callsite in `runPipeline` (line 300) |
| `cmd/dotd/compose_cmd.go` | Modify | Replace 2 `pipeline.Filter` callsites (lines 41, 70) |
| `cmd/dotd/dag_cmd.go` | Modify | Replace 1 `pipeline.Filter` callsite (line 32) |
| `cmd/dotd/list_cmd.go` | Modify | Replace 1 `pipeline.Filter` callsite (line 57) |
| `cmd/dotd/package.go` | Modify | Replace 3 `pipeline.Filter` callsites (lines 39, 81, 110) |
| `cmd/dotd/bundle.go` | Modify | Replace 1 `pipeline.Filter` callsite (line 54) |

---

## Task 0: Create feature branch

- [ ] **Step 1: Branch from main**

```bash
git checkout main && git pull && git checkout -b feature/claude-tty-missing-key-prompt
```

---

## Task 1: `pipeline.CollectMissingKeys`

**Files:**
- Modify: `internal/pipeline/filter.go`
- Modify: `internal/pipeline/filter_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/pipeline/filter_test.go`:

```go
func TestCollectMissingKeys_SingleMissing(t *testing.T) {
	nodes := []RawNode{makeNode("a", "context=work")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "context" {
		t.Errorf("expected [context], got %v", got)
	}
}

func TestCollectMissingKeys_AndBothMissing(t *testing.T) {
	// Both sides of AND must be found — no short-circuit.
	nodes := []RawNode{makeNode("a", "context=work AND machine=laptop")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 missing keys, got %v", got)
	}
	keys := map[string]bool{got[0]: true, got[1]: true}
	if !keys["context"] || !keys["machine"] {
		t.Errorf("expected context and machine, got %v", got)
	}
}

func TestCollectMissingKeys_Dedup(t *testing.T) {
	// Same key referenced in two nodes — returned once.
	nodes := []RawNode{
		makeNode("a", "context=work"),
		makeNode("b", "context=personal"),
	}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "context" {
		t.Errorf("expected [context] once, got %v", got)
	}
}

func TestCollectMissingKeys_Nonemissing(t *testing.T) {
	nodes := []RawNode{makeNode("a", "context=work")}
	got, err := CollectMissingKeys(nodes, map[string]string{"context": "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCollectMissingKeys_EmptyWhenSkipped(t *testing.T) {
	nodes := []RawNode{makeNode("base", "")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty for empty-when node, got %v", got)
	}
}

func TestCollectMissingKeys_ParseError(t *testing.T) {
	nodes := []RawNode{makeNode("bad", "!!invalid!!")}
	_, err := CollectMissingKeys(nodes, map[string]string{})
	if err == nil {
		t.Error("expected parse error, got nil")
	}
}

func TestCollectMissingKeys_PartiallySet(t *testing.T) {
	// context set, machine missing.
	nodes := []RawNode{makeNode("a", "context=work AND machine=laptop")}
	got, err := CollectMissingKeys(nodes, map[string]string{"context": "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "machine" {
		t.Errorf("expected [machine], got %v", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/pipeline/ -run TestCollectMissingKeys -v
```

Expected: `FAIL — CollectMissingKeys undefined`

- [ ] **Step 3: Implement `CollectMissingKeys`**

Add to `internal/pipeline/filter.go` (after the `Filter` function). Add `"errors"` to the import block if not present — check existing imports first.

```go
// CollectMissingKeys returns env keys referenced by predicate expressions across
// all nodes that are absent from env. Uses AST key extraction (no evaluation),
// so AND/OR short-circuiting cannot hide keys. Returns keys in encounter order.
func CollectMissingKeys(nodes []RawNode, env map[string]string) ([]string, error) {
	seen := map[string]bool{}
	var keys []string
	for _, n := range nodes {
		if n.EffectiveWhen == "" {
			continue
		}
		parsed, err := predicate.Parse(n.EffectiveWhen)
		if err != nil {
			return nil, fmt.Errorf("filter: node %q: %w", n.LogicalName, err)
		}
		for _, k := range parsed.Keys() {
			if _, ok := env[k]; !ok && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/pipeline/ -run TestCollectMissingKeys -v
```

Expected: all 7 tests `PASS`

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages `ok`

- [ ] **Step 6: Commit**

```bash
git add internal/pipeline/filter.go internal/pipeline/filter_test.go
git commit -m "feat(pipeline): add CollectMissingKeys via AST key scan"
```

---

## Task 2: `filterWithPrompt` — non-TTY path

**Files:**
- Create: `cmd/dotd/filter_prompt.go`
- Create: `cmd/dotd/filter_prompt_test.go`

The non-TTY and zero-missing-keys paths are unit-testable. The interactive prompt path is not — covered by manual test in Task 4.

- [ ] **Step 1: Write failing tests**

Create `cmd/dotd/filter_prompt_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/pipeline"
)

func makeFilterNode(name, when string) pipeline.RawNode {
	return pipeline.RawNode{
		Path:          "/dots/" + name,
		LogicalName:   name,
		EffectiveWhen: when,
	}
}

func TestFilterWithPrompt_NonTTY_MissingKey_ReturnsAnnotatedError(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("a", "context=work")}
	resolved := map[string]string{} // context missing

	_, err := filterWithPrompt(nodes, resolved, false)
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected error to mention key 'context', got: %v", err)
	}
}

func TestFilterWithPrompt_NonTTY_NoMissingKeys_ReturnsActiveNodes(t *testing.T) {
	nodes := []pipeline.RawNode{
		makeFilterNode("base", ""),
		makeFilterNode("work", "context=work"),
		makeFilterNode("personal", "context=personal"),
	}
	resolved := map[string]string{"context": "work"}

	active, err := filterWithPrompt(nodes, resolved, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active nodes (base + work), got %d", len(active))
	}
}

func TestFilterWithPrompt_TTY_NoMissingKeys_DoesNotPrompt(t *testing.T) {
	// isTTY=true but no missing keys — should proceed without prompting.
	nodes := []pipeline.RawNode{makeFilterNode("base", "")}
	resolved := map[string]string{}

	active, err := filterWithPrompt(nodes, resolved, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active node, got %d", len(active))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./cmd/dotd/ -run TestFilterWithPrompt -v
```

Expected: `FAIL — filterWithPrompt undefined`

- [ ] **Step 3: Create `cmd/dotd/filter_prompt.go` with non-TTY path**

```go
package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/pipeline"
)

// filterWithPrompt wraps pipeline.Filter with TTY-aware missing-key prompting.
// Non-TTY: identical to Filter + annotateKeyError.
// TTY with missing keys: prompts for all missing keys, then runs Filter with augmented env.
func filterWithPrompt(nodes []pipeline.RawNode, resolved map[string]string, isTTY bool) ([]pipeline.RawNode, error) {
	if !isTTY {
		active, err := pipeline.Filter(nodes, resolved)
		return active, annotateKeyError(err)
	}

	missing, err := pipeline.CollectMissingKeys(nodes, resolved)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		return pipeline.Filter(nodes, resolved)
	}

	filled, err := promptMissingKeys(missing)
	if err != nil {
		return nil, err
	}

	printPersistHint(os.Stderr, filled)

	augmented := maps.Clone(resolved)
	for k, v := range filled {
		augmented[k] = v
	}

	active, err := pipeline.Filter(nodes, augmented)
	return active, annotateKeyError(err)
}

func promptMissingKeys(keys []string) (map[string]string, error) {
	vals := make([]string, len(keys))
	fields := make([]huh.Field, len(keys))
	for i, k := range keys {
		i, k := i, k
		fields[i] = huh.NewInput().
			Title(fmt.Sprintf("env key %q is not set", k)).
			Value(&vals[i]).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("value required")
				}
				return nil
			})
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return nil, err
	}
	filled := make(map[string]string, len(keys))
	for i, k := range keys {
		filled[k] = vals[i]
	}
	return filled, nil
}

func printPersistHint(w io.Writer, filled map[string]string) {
	fmt.Fprintln(w, "\nHint: to persist, add to env.yaml:")
	for k, v := range filled {
		fmt.Fprintf(w, "  %s: %s\n", k, v)
	}
}

// isTTYStdin returns true when os.Stdin is an interactive terminal.
func isTTYStdin() bool {
	return term.IsTerminal(os.Stdin.Fd())
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./cmd/dotd/ -run TestFilterWithPrompt -v
```

Expected: all 3 tests `PASS`

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages `ok`

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/filter_prompt.go cmd/dotd/filter_prompt_test.go
git commit -m "feat(cmd): add filterWithPrompt with TTY-aware missing key prompting"
```

---

## Task 3: Replace `pipeline.Filter` callsites

**Files:**
- Modify: `cmd/dotd/main.go` (line 300)
- Modify: `cmd/dotd/compose_cmd.go` (lines 41, 70)
- Modify: `cmd/dotd/dag_cmd.go` (line 32)
- Modify: `cmd/dotd/list_cmd.go` (line 57)
- Modify: `cmd/dotd/package.go` (lines 39, 81, 110)
- Modify: `cmd/dotd/bundle.go` (line 54)

The `annotateKeyError` calls that wrap `resolveEnv` errors (not `Filter`) are left untouched.

**Pattern for single-return-value functions** (compose, dag, list, package, bundle):

```go
// Before:
active, err := pipeline.Filter(nodes, resolved)
if err != nil {
    return annotateKeyError(err)
}

// After:
active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
if err != nil {
    return err
}
```

**Pattern for two-return-value functions** (main.go `runPipeline`):

```go
// Before:
active, err := pipeline.Filter(nodes, resolved)
if err != nil {
    return nil, annotateKeyError(err)
}

// After:
active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
if err != nil {
    return nil, err
}
```

- [ ] **Step 1: Update `cmd/dotd/main.go` (line ~300)**

Replace in `runPipeline`:

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return nil, err
	}
```

Remove `"github.com/rocne/dot-dagger/internal/pipeline"` from imports in main.go only if it is no longer referenced elsewhere in that file — check first with `grep "pipeline\." cmd/dotd/main.go`. It likely still is (Walk, Order, Act, etc.), so leave the import.

- [ ] **Step 2: Update `cmd/dotd/compose_cmd.go` (lines ~41 and ~70)**

Two separate `pipeline.Filter` calls — each gets the same replacement:

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return err
	}
```

- [ ] **Step 3: Update `cmd/dotd/dag_cmd.go` (line ~32)**

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return err
	}
```

- [ ] **Step 4: Update `cmd/dotd/list_cmd.go` (line ~57)**

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return err
	}
```

- [ ] **Step 5: Update `cmd/dotd/package.go` (lines ~39, ~81, ~110)**

Three replacements, same pattern:

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return err
	}
```

- [ ] **Step 6: Update `cmd/dotd/bundle.go` (line ~54)**

```go
	active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
	if err != nil {
		return err
	}
```

- [ ] **Step 7: Remove unused `pipeline.Filter` imports**

Check each modified file: if `pipeline.Filter` was the only `pipeline.*` call in that file, the import needs updating. Run:

```bash
go build ./cmd/dotd/ 2>&1
```

Fix any `imported and not used` errors by removing the `pipeline` import from the affected file.

- [ ] **Step 8: Run full test suite**

```bash
go test ./...
```

Expected: all packages `ok`

- [ ] **Step 9: Commit**

```bash
git add cmd/dotd/main.go cmd/dotd/compose_cmd.go cmd/dotd/dag_cmd.go \
        cmd/dotd/list_cmd.go cmd/dotd/package.go cmd/dotd/bundle.go
git commit -m "refactor(cmd): replace pipeline.Filter callsites with filterWithPrompt"
```

---

## Task 4: Manual smoke test

- [ ] **Step 1: Build**

```bash
go build ./cmd/dotd/ -o /tmp/dotd-test
```

- [ ] **Step 2: Run against a dotfiles repo with a missing key**

Set up a test: temporarily rename your `env.yaml` so a required key is absent, then run:

```bash
/tmp/dotd-test apply
```

Expected: `huh` form appears prompting for each missing key. After filling values, `apply` proceeds normally. Persist hint printed to stderr immediately after the prompt.

- [ ] **Step 3: Verify non-TTY path unchanged**

```bash
echo "" | /tmp/dotd-test apply 2>&1
```

Expected: same `predicate: env key "X" not set` error + hint as before (no prompt).

- [ ] **Step 4: Open PR**

```bash
gh pr view 2>/dev/null && echo "PR already open" || gh pr create \
  --title "feat(cmd): TTY-aware missing key prompt (M3)" \
  --body "$(cat <<'EOF'
## Summary

- When stdin is a TTY and predicate env keys are missing, prompt for values interactively instead of halting
- Values used for current run only; persist hint (YAML snippet) printed to stderr after prompt
- Non-TTY behavior unchanged — same error + hint as before

## Changes

- `pipeline.CollectMissingKeys`: AST-based key scan across all nodes (no short-circuit risk)
- `cmd/dotd/filter_prompt.go`: `filterWithPrompt` wraps Filter with huh prompts
- All `pipeline.Filter` callsites in command files replaced with `filterWithPrompt`

## Test plan

- [ ] `go test ./...` passes
- [ ] Manual: missing key in TTY → prompt appears, proceed after fill, hint shown
- [ ] Manual: missing key piped → error + hint as before (no prompt)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
