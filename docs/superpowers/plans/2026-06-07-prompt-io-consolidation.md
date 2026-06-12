# Prompt I/O Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `prompts.go` the single owner of all huh interactions so every interactive command is testable via `cmd.SetIn(strings.NewReader(...))` without a real terminal.

**Architecture:** Add `isTTY(io.Reader)`, `newHuhForm(cmd, fields...)`, and typed helpers (`promptMenu`, `promptSelect`, `promptInput`, `promptBool`, `promptInputs`) to `prompts.go`. All helpers wire cobra I/O and enable huh accessible mode automatically when stdin is not a terminal. Remove direct huh imports from `adopt.go`, `annotate_cmd.go`, and `filter_prompt.go`. In accessible mode huh renders numbered menus and line-buffered text input — fully driveable by piped stdin in tests and CI.

**Tech Stack:** Go, `github.com/charmbracelet/huh v1.0.0` (accessible mode API: `Form.WithAccessible`, `WithInput`, `WithOutput`), cobra (`cmd.InOrStdin()`, `cmd.OutOrStdout()`), `github.com/charmbracelet/x/term`.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/dotd/prompts.go` | Modify | Add `isTTY`, `newHuhForm`, and all typed huh helpers; receive `errUserAborted` moved here |
| `cmd/dotd/prompts_test.go` | Modify | Add `TestIsTTY_*` unit tests |
| `cmd/dotd/filter_prompt.go` | Modify | Remove huh import, `isTTYStdin`, `prompter` var, `errUserAborted` def; change `filterWithPrompt` signature; use `promptInputs`; fix `os.Stderr` |
| `cmd/dotd/filter_prompt_test.go` | Modify | Remove `prompter` stubs; update `filterWithPrompt` call signatures; add stdin-injected TTY test |
| `cmd/dotd/main.go` | Modify | Update 2 `filterWithPrompt` call sites |
| `cmd/dotd/adopt.go` | Modify | Remove huh import; delete `promptAdoptConfirm`; use `promptBool`; use `isTTY(cmd.InOrStdin())` |
| `cmd/dotd/annotate_cmd.go` | Modify | Remove huh import; remove TTY guard; replace all huh calls with helpers |
| `cmd/dotd/integration_test.go` | Modify | Replace `TestAnnotate_NonTTY`; add 4 wizard tests using `runWithStdin` |
| `test/e2e/annotate.sh` | Create | E2E shell script for annotate command |
| `test/e2e/Dockerfile` | Modify | Add `COPY annotate.sh` |
| `test/e2e/Dockerfile.local` | Modify | Add `COPY annotate.sh` |
| `test/run-e2e.sh` | Modify | Add `run_test annotate.sh` |

---

## Task 1: Add canonical huh helpers to `prompts.go`

**Files:**
- Modify: `cmd/dotd/prompts.go`
- Modify: `cmd/dotd/annotate_cmd.go` (one call-site fix to prevent compile break)
- Modify: `cmd/dotd/prompts_test.go`

This task adds all new helpers and moves `errUserAborted` to `prompts.go`. The old `promptMenu(options []string)` is replaced by `promptMenu(cmd, title, options)` — its only call site in `annotate_cmd.go` is updated in the same commit to keep the build green. The rest of `annotate_cmd.go` still imports huh (cleaned up in Task 4).

- [ ] **Step 1: Write the failing tests in `prompts_test.go`**

Add `"os"` to the existing import block in `cmd/dotd/prompts_test.go`, then add these tests at the bottom of the file:

```go
func TestIsTTY_StringsReader_ReturnsFalse(t *testing.T) {
	if isTTY(strings.NewReader("")) {
		t.Error("strings.Reader is not a TTY; isTTY should return false")
	}
}

func TestIsTTY_DevNull_ReturnsFalse(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTTY(f) {
		t.Error("/dev/null is not a TTY; isTTY should return false")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/... -run 'TestIsTTY' -v
```

Expected: compile error — `isTTY` undefined.

- [ ] **Step 3: Update `cmd/dotd/prompts.go`**

Replace the entire file with this content:

```go
package main

// Prompt conventions:
//   - All prompts default to a SAFE choice when stdin is EOF (no, cancel, skip).
//   - When the user is interactive, [Y/n] vs [y/N] indicates the Enter-default.
//   - [Y/n]: Enter → yes.  [y/N]: Enter → no.
//   - Never auto-accept a destructive or filesystem-mutating action on EOF.
//
// huh helpers (promptMenu, promptSelect, promptInput, promptBool, promptInputs):
//   - All route through cmd.InOrStdin() / cmd.OutOrStdout().
//   - Non-TTY contexts (tests, CI) automatically use huh accessible mode:
//     numbered menus, line-buffered text. Driveable by cmd.SetIn(strings.NewReader(...)).
//   - This file is the only file that imports charmbracelet/huh.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

// errUserAborted is returned when the user cancels an interactive prompt.
// main() maps this sentinel to a clean exit-1 with "cancelled" on stderr,
// avoiding the noisy "Error: user aborted" that Cobra would otherwise print.
var errUserAborted = errors.New("user aborted")

// isTTY reports whether r is an interactive terminal.
// Returns false for any reader without an fd — e.g. strings.Reader in tests,
// piped stdin in CI. Use isTTY(cmd.InOrStdin()) instead of os.Stdin.Fd() directly.
func isTTY(r io.Reader) bool {
	if f, ok := r.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

// newHuhForm returns a huh.Form wired to cmd's stdin and stdout.
// When stdin is not a terminal (tests, CI, piped input) it automatically
// enables accessible mode: plain numbered menus and line-buffered text input.
func newHuhForm(cmd *cobra.Command, fields ...huh.Field) *huh.Form {
	r := cmd.InOrStdin()
	return huh.NewForm(huh.NewGroup(fields...)).
		WithAccessible(!isTTY(r)).
		WithInput(r).
		WithOutput(cmd.OutOrStdout())
}

// promptMenu presents a numbered selection menu and returns the zero-based index
// of the chosen option. Callers typically make the last option "Done".
func promptMenu(cmd *cobra.Command, title string, options []string) (int, error) {
	var chosen string
	err := newHuhForm(cmd,
		huh.NewSelect[string]().
			Title(title).
			Options(huh.NewOptions(options...)...).
			Value(&chosen),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return 0, errUserAborted
	}
	if err != nil {
		return 0, err
	}
	for i, o := range options {
		if o == chosen {
			return i, nil
		}
	}
	return 0, fmt.Errorf("promptMenu: unknown selection %q", chosen)
}

// promptSelect presents a labeled selection with description and returns the chosen value.
func promptSelect(cmd *cobra.Command, title, desc string, options []string) (string, error) {
	var chosen string
	err := newHuhForm(cmd,
		huh.NewSelect[string]().
			Title(title).
			Description(desc).
			Options(huh.NewOptions(options...)...).
			Value(&chosen),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return "", errUserAborted
	}
	return chosen, err
}

// promptInput shows a text field pre-filled with prefill and returns the trimmed value.
// An empty result (user clears the field) is allowed; callers treat it as "remove".
// validate is called on non-empty input; pass nil to skip validation.
func promptInput(cmd *cobra.Command, title, desc, prefill string, validate func(string) error) (string, error) {
	val := prefill
	err := newHuhForm(cmd,
		huh.NewInput().
			Title(title).
			Description(desc).
			Value(&val).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return nil // empty = remove; always valid
				}
				if validate != nil {
					return validate(s)
				}
				return nil
			}),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return "", errUserAborted
	}
	return strings.TrimSpace(val), err
}

// promptBool shows a yes/no confirm pre-set to initial and returns the chosen value.
func promptBool(cmd *cobra.Command, title, desc, affirm, neg string, initial bool) (bool, error) {
	val := initial
	err := newHuhForm(cmd,
		huh.NewConfirm().
			Title(title).
			Description(desc).
			Affirmative(affirm).
			Negative(neg).
			Value(&val),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return false, errUserAborted
	}
	return val, err
}

// inputPrompt describes a single text input field for use with promptInputs.
type inputPrompt struct {
	Key      string
	Title    string
	Validate func(string) error
}

// promptInputs presents a form with one text field per prompt entry and returns
// a map of Key → trimmed value. An empty value means the field was left blank.
func promptInputs(cmd *cobra.Command, prompts []inputPrompt) (map[string]string, error) {
	vals := make([]string, len(prompts))
	fields := make([]huh.Field, len(prompts))
	for i, p := range prompts {
		i, p := i, p
		validate := p.Validate
		fields[i] = huh.NewInput().
			Title(p.Title).
			Value(&vals[i]).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("value required")
				}
				if validate != nil {
					return validate(s)
				}
				return nil
			})
	}
	err := newHuhForm(cmd, fields...).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, errUserAborted
	}
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(prompts))
	for i, p := range prompts {
		result[p.Key] = strings.TrimSpace(vals[i])
	}
	return result, nil
}

// promptConfirm prints "Proceed? [y/N]: ", reads a line, and returns true only
// on "y" or "yes". Any other input (including empty / Enter or EOF) prints
// "cancelled" and returns false — callers should return nil when false.
func promptConfirm(out io.Writer, r io.Reader) bool {
	fmt.Fprint(out, "\nProceed? [y/N]: ")
	ans, _ := bufio.NewReader(r).ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans != "y" && ans != "yes" {
		ui.Skipf(out, "cancelled")
		return false
	}
	return true
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, ui.Skip(defaultVal))
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// promptYN prints "msg [Y/n]: " and returns true unless user types n/no.
// Interactive: empty input (Enter) defaults to yes.
// Non-interactive: EOF returns false (safe default — never auto-accept on closed stdin).
func promptYN(w io.Writer, reader *bufio.Reader, msg string) (bool, error) {
	fmt.Fprintf(w, "  %s [Y/n]: ", msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, nil // EOF → safe default: no
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes", nil
}

// printField prints a bold field label and a faint description, then a blank line.
func printField(w io.Writer, label, desc string) {
	fmt.Fprintf(w, "\n  %s\n", ui.Key(label))
	fmt.Fprintf(w, "  %s\n", ui.Skip(desc))
}

// fieldPrompt returns the prompt text used after a printField call.
func fieldPrompt() string {
	return "  " + ui.Arrow("›")
}
```

- [ ] **Step 4: Remove `errUserAborted` from `filter_prompt.go`**

`prompts.go` now defines `errUserAborted`. The existing definition in `filter_prompt.go` creates a duplicate symbol — find and delete this block from `cmd/dotd/filter_prompt.go`:

```go
// errUserAborted is returned when the user cancels an interactive prompt.
// main() maps this sentinel to a clean exit-1 with "cancelled" on stderr,
// avoiding the noisy "Error: user aborted" that Cobra would otherwise print.
var errUserAborted = errors.New("user aborted")
```

(`filter_prompt.go` is rewritten in Task 2, but it must compile now. `prompts.go` owns the definition from this commit forward.)

- [ ] **Step 5: Fix the one broken `promptMenu` call in `annotate_cmd.go`**

In `cmd/dotd/annotate_cmd.go`, find the line:

```go
		idx, err := promptMenu(options)
```

Replace with:

```go
		idx, err := promptMenu(cmd, "Select annotation", options)
```

- [ ] **Step 6: Verify it compiles**

```bash
cd /home/rocne/git/dot-dagger && go build ./cmd/dotd/...
```

Expected: no output, exit 0.

- [ ] **Step 7: Run all tests**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/... -v -run 'TestIsTTY|TestPromptYN|TestPromptConfirm'
```

Expected: all pass including the two new `TestIsTTY_*` tests.

- [ ] **Step 8: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/prompts.go cmd/dotd/prompts_test.go cmd/dotd/annotate_cmd.go cmd/dotd/filter_prompt.go
git commit -m "refactor(prompts): add canonical huh helpers with cobra I/O routing"
```

---

## Task 2: Refactor `filter_prompt.go` — remove huh, fix `os.Stderr`

**Files:**
- Modify: `cmd/dotd/filter_prompt.go`
- Modify: `cmd/dotd/filter_prompt_test.go`
- Modify: `cmd/dotd/main.go`

`filterWithPrompt` keeps a `tty bool` parameter (passed from main.go at the call site) so tests can exercise the TTY branch with a controlled bool. Internally it uses `cmd` for I/O — the `tty` param only controls which code branch is taken.

- [ ] **Step 1: Write the updated tests first**

Replace the entire contents of `cmd/dotd/filter_prompt_test.go` with:

```go
package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func makeFilterNode(name, when string) pipeline.RawNode {
	return pipeline.RawNode{
		Path:          "/dots/" + name,
		LogicalName:   name,
		EffectiveWhen: when,
	}
}

// testCmd returns a cobra.Command wired to the given stdin.
// stdout and stderr are discarded so test output stays clean.
func testCmd(stdin io.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetIn(stdin)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

func TestFilterWithPrompt_NonTTY_MissingKey_ReturnsAnnotatedError(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("a", "context=work")}
	resolved := map[string]string{} // context missing

	_, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, false)
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

	active, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active nodes (base + work), got %d", len(active))
	}
}

func TestFilterWithPrompt_TTY_NoMissingKeys_DoesNotPrompt(t *testing.T) {
	nodes := []pipeline.RawNode{makeFilterNode("base", "")}
	resolved := map[string]string{}

	active, err := filterWithPrompt(testCmd(strings.NewReader("")), nodes, resolved, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active node, got %d", len(active))
	}
}

// TestFilterWithPrompt_TTY_HappyPath verifies that when the user provides values
// for missing keys via accessible-mode stdin, those values augment the env and
// the correct nodes filter through.
func TestFilterWithPrompt_TTY_HappyPath(t *testing.T) {
	nodes := []pipeline.RawNode{
		makeFilterNode("base", ""),
		makeFilterNode("linux-work", "os=linux"),
		makeFilterNode("macos-only", "os=macos"),
		makeFilterNode("work-context", "context=work"),
	}
	resolved := map[string]string{} // both "os" and "context" missing

	// Accessible mode: huh prompts for each missing key in order.
	// promptMissingKeys calls CollectMissingKeys then promptInputs.
	// CollectMissingKeys returns keys in encounter order (not sorted).
	// Nodes iterate: "linux-work" has os=linux → "os" collected first;
	// "work-context" has context=work → "context" collected second.
	// Input: "linux" for os, "work" for context.
	stdin := strings.NewReader("linux\nwork\n")

	active, err := filterWithPrompt(testCmd(stdin), nodes, resolved, true)
	if err != nil {
		t.Fatalf("filterWithPrompt error = %v", err)
	}

	names := map[string]bool{}
	for _, n := range active {
		names[n.LogicalName] = true
	}
	if !names["base"] {
		t.Error("expected 'base' (unconditional) in active nodes")
	}
	if !names["linux-work"] {
		t.Error("expected 'linux-work' in active nodes")
	}
	if !names["work-context"] {
		t.Error("expected 'work-context' in active nodes")
	}
	if names["macos-only"] {
		t.Error("expected 'macos-only' NOT in active nodes")
	}
}

func TestPrintPersistHint(t *testing.T) {
	filled := map[string]string{
		"os":      "linux",
		"context": "work",
	}

	var buf bytes.Buffer
	printPersistHint(&buf, filled)
	out := buf.String()

	if !strings.Contains(out, "os") {
		t.Errorf("expected 'os' key in persist hint output: %q", out)
	}
	if !strings.Contains(out, "linux") {
		t.Errorf("expected 'linux' value in persist hint output: %q", out)
	}
	if !strings.Contains(out, "context") {
		t.Errorf("expected 'context' key in persist hint output: %q", out)
	}
	if !strings.Contains(out, "work") {
		t.Errorf("expected 'work' value in persist hint output: %q", out)
	}
	if !strings.Contains(out, "env.yaml") {
		t.Errorf("expected 'env.yaml' in persist hint output: %q", out)
	}
}
```

> **Note:** `TestFilterWithPrompt_TTY_HappyPath` pipes `"linux\nwork\n"` because `pipeline.CollectMissingKeys` returns keys in **encounter order** (not sorted). The test nodes have `os=linux` before `context=work`, so "os" is collected first. If the node order in the test changes, the input order must change too.

- [ ] **Step 2: Run the tests — verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/... -run 'TestFilter|TestPrintPersist' -v
```

Expected: compile errors — `filterWithPrompt` signature mismatch, `testCmd` undefined, `prompter` removed.

- [ ] **Step 3: Confirm key order for `TestFilterWithPrompt_TTY_HappyPath`**

```bash
cd /home/rocne/git/dot-dagger && grep -n "encounter\|sort\|Sort" internal/pipeline/filter.go | head -10
```

Expected: the comment confirms encounter order (no sort call). The test input `"linux\nwork\n"` assumes "os" is encountered before "context" based on the test node slice order. This is already correct — no changes needed.

- [ ] **Step 4: Rewrite `cmd/dotd/filter_prompt.go`**

Replace the entire file:

```go
package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// filterWithPrompt wraps pipeline.Filter with TTY-aware missing-key prompting.
// tty=false: identical to Filter + annotateKeyError (non-interactive path).
// tty=true with missing keys: prompts for all missing keys via cmd's I/O,
// then re-runs Filter with the augmented env.
// Call site: filterWithPrompt(cmd, nodes, resolved, isTTY(cmd.InOrStdin()))
func filterWithPrompt(cmd *cobra.Command, nodes []pipeline.RawNode, resolved map[string]string, tty bool) ([]pipeline.RawNode, error) {
	if !tty {
		active, err := pipeline.Filter(nodes, resolved)
		return active, annotateKeyError(err)
	}

	missing, err := pipeline.CollectMissingKeys(nodes, resolved)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		active, err := pipeline.Filter(nodes, resolved)
		return active, annotateKeyError(err)
	}

	filled, err := promptMissingKeys(cmd, missing)
	if err != nil {
		return nil, err
	}

	printPersistHint(cmd.ErrOrStderr(), filled)

	augmented := maps.Clone(resolved)
	for k, v := range filled {
		augmented[k] = v
	}

	active, err := pipeline.Filter(nodes, augmented)
	return active, annotateKeyError(err)
}

func promptMissingKeys(cmd *cobra.Command, keys []string) (map[string]string, error) {
	prompts := make([]inputPrompt, len(keys))
	for i, k := range keys {
		prompts[i] = inputPrompt{
			Key:   k,
			Title: fmt.Sprintf("env key %q is not set", k),
		}
	}
	return promptInputs(cmd, prompts)
}

func printPersistHint(w io.Writer, filled map[string]string) {
	out, err := yaml.Marshal(filled)
	if err != nil {
		fmt.Fprintf(w, "\nHint: to persist, add to %s:\n", ecosystem.EnvFileName)
		for k, v := range filled {
			fmt.Fprintf(w, "  %s: %s\n", k, v)
		}
		return
	}
	fmt.Fprintf(w, "\nHint: to persist, add to %s:\n", ecosystem.EnvFileName)
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

// isTTYStdin is a bridge shim keeping adopt.go and annotate_cmd.go compiling
// until Tasks 3 and 4 migrate those callers. Removed in Task 4.
// Deprecated: use isTTY(cmd.InOrStdin()) instead.
func isTTYStdin() bool { return isTTY(os.Stdin) }
```

- [ ] **Step 5: Update `cmd/dotd/main.go` — fix 2 call sites**

Find both occurrences of:
```go
active, err := filterWithPrompt(nodes, resolved, isTTYStdin())
```

Replace each with:
```go
active, err := filterWithPrompt(cmd, nodes, resolved, isTTY(cmd.InOrStdin()))
```

- [ ] **Step 6: Verify it compiles**

```bash
cd /home/rocne/git/dot-dagger && go build ./cmd/dotd/...
```

Expected: no output, exit 0.

- [ ] **Step 7: Run all tests**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/...
```

Expected: all pass. Pay attention to `TestFilterWithPrompt_TTY_HappyPath` — if it fails with a key order issue, revisit Step 3 and fix the test input order.

- [ ] **Step 8: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/filter_prompt.go cmd/dotd/filter_prompt_test.go cmd/dotd/main.go
git commit -m "refactor(filter_prompt): remove huh, use promptInputs, fix os.Stderr"
```

---

## Task 3: Refactor `adopt.go` — remove direct huh usage

**Files:**
- Modify: `cmd/dotd/adopt.go`

- [ ] **Step 1: Verify existing adopt tests pass before touching anything**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/... -run 'TestAdopt' -v
```

Expected: all 5 adopt tests pass. (All use `--yes`, so they skip the huh prompt — they will still pass after this refactor.)

- [ ] **Step 2: Delete `promptAdoptConfirm` and replace its call site**

In `cmd/dotd/adopt.go`:

Find and **delete** the entire function:
```go
func promptAdoptConfirm(src, destRel string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Adopt %s → %s and replace with symlink?", src, destRel)).
		Affirmative("Yes").
		Negative("No, cancel").
		Value(&confirmed).
		Run()
	if err != nil {
		return false, fmt.Errorf("adopt: prompt: %w", err)
	}
	return confirmed, nil
}
```

Find the call site (roughly around line 91–95):
```go
nonInteractive := yes || !isTTYStdin()
if !nonInteractive {
    confirmed, promptErr := promptAdoptConfirm(srcAbs, destRel)
    if promptErr != nil {
        return fmt.Errorf("adopt: %w", promptErr)
    }
    if !confirmed {
        return nil
    }
}
```

Replace with:
```go
nonInteractive := yes || !isTTY(cmd.InOrStdin())
if !nonInteractive {
    confirmed, err := promptBool(cmd,
        fmt.Sprintf("Adopt %s → %s and replace with symlink?", srcAbs, destRel),
        "", "Yes", "No, cancel", false)
    if err != nil {
        return fmt.Errorf("adopt: %w", err)
    }
    if !confirmed {
        return nil
    }
}
```

- [ ] **Step 3: Remove the `huh` import from `adopt.go`**

Find the import block and remove `"github.com/charmbracelet/huh"`.

- [ ] **Step 4: Verify it compiles**

```bash
cd /home/rocne/git/dot-dagger && go build ./cmd/dotd/...
```

Expected: no output, exit 0. The bridge shim `isTTYStdin()` stays in `filter_prompt.go` for now — `annotate_cmd.go` still references it. It is removed in Task 4 once the TTY guard is gone.

- [ ] **Step 5: Run all tests**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/adopt.go
git commit -m "refactor(adopt): remove direct huh usage, use promptBool"
```

---

## Task 4: Refactor `annotate_cmd.go` — remove all direct huh usage

**Files:**
- Modify: `cmd/dotd/annotate_cmd.go`

Remove the TTY guard entirely — the command now works in non-TTY contexts via accessible mode. Replace all four `huh.New*().Run()` calls with the helpers from `prompts.go`.

- [ ] **Step 1: Remove the TTY guard**

Find and delete:
```go
if !isTTYStdin() {
    return fmt.Errorf("dotd annotate requires an interactive terminal")
}
```

- [ ] **Step 2: Remove the `isTTYStdin` bridge shim from `filter_prompt.go`**

`annotate_cmd.go` was the last caller of `isTTYStdin()`. Now that the TTY guard is gone, delete this block from `cmd/dotd/filter_prompt.go`:

```go
// isTTYStdin is a bridge shim keeping adopt.go compiling until Task 3 removes it.
// Deprecated: use isTTY(cmd.InOrStdin()) instead.
func isTTYStdin() bool { return isTTY(os.Stdin) }
```

Also remove `"os"` from `filter_prompt.go`'s import block if it is now unused.

- [ ] **Step 3: Replace KindBool prompt**

Find:
```go
		case annotation.KindBool:
			set := len(entries) > 0 // pre-populate with current state
			if err := huh.NewConfirm().
				Title(t.Label() + "?").
				Description(t.Description()).
				Affirmative("Set").
				Negative("Remove").
				Value(&set).
				Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return errUserAborted
				}
				return fmt.Errorf("annotate: confirm: %w", err)
			}
```

Replace with:
```go
		case annotation.KindBool:
			set, err := promptBool(cmd, t.Label()+"?", t.Description(), "Set", "Remove", len(entries) > 0)
			if err != nil {
				return fmt.Errorf("annotate: confirm: %w", err)
			}
```

- [ ] **Step 4: Replace KindChoice prompt**

Find:
```go
		case annotation.KindChoice:
			opts := append(append([]string{}, t.Options()...), "none")
			var chosen string
			if err := huh.NewSelect[string]().
				Title(t.Label()).
				Description(t.Description()).
				Options(huh.NewOptions(opts...)...).
				Value(&chosen).
				Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return errUserAborted
				}
				return fmt.Errorf("annotate: select: %w", err)
			}
```

Replace with:
```go
		case annotation.KindChoice:
			opts := append(append([]string{}, t.Options()...), "none")
			chosen, err := promptSelect(cmd, t.Label(), t.Description(), opts)
			if err != nil {
				return fmt.Errorf("annotate: select: %w", err)
			}
```

- [ ] **Step 5: Replace KindText prompt**

Find:
```go
		case annotation.KindText:
			val := ""
			if len(entries) == 1 {
				val = entries[0].Value
			}
			if err := huh.NewInput().
				Title(t.Label()).
				Description(t.Description() + "  (clear the field to remove)").
				Value(&val).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return nil // empty = remove; always allowed
					}
					return t.Validate(s)
				}).
				Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return errUserAborted
				}
				return fmt.Errorf("annotate: input: %w", err)
			}
			val = strings.TrimSpace(val)
```

Replace with:
```go
		case annotation.KindText:
			prefill := ""
			if len(entries) == 1 {
				prefill = entries[0].Value
			}
			val, err := promptInput(cmd, t.Label(), t.Description()+"  (clear the field to remove)", prefill, t.Validate)
			if err != nil {
				return fmt.Errorf("annotate: input: %w", err)
			}
```

- [ ] **Step 6: Replace the final confirm prompt**

Find:
```go
	var confirmed bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Write these annotations to %s?", filepath.Base(absFile))).
		Affirmative("Yes").
		Negative("No, cancel").
		Value(&confirmed).
		Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errUserAborted
		}
		return fmt.Errorf("annotate: confirm: %w", err)
	}
```

Replace with:
```go
	confirmed, err := promptBool(cmd,
		fmt.Sprintf("Write these annotations to %s?", filepath.Base(absFile)),
		"", "Yes", "No, cancel", false)
	if err != nil {
		return fmt.Errorf("annotate: confirm: %w", err)
	}
```

- [ ] **Step 7: Remove the `huh` import and any now-unused imports**

Remove `"github.com/charmbracelet/huh"` from the import block of `annotate_cmd.go`. Also remove `"errors"` if it is now unused (check whether any remaining `errors.Is` or `errors.New` calls reference it).

- [ ] **Step 8: Verify it compiles**

```bash
cd /home/rocne/git/dot-dagger && go build ./cmd/dotd/...
```

Expected: no output, exit 0.

- [ ] **Step 9: Confirm no file imports huh except `prompts.go`**

```bash
grep -rn '"github.com/charmbracelet/huh"' /home/rocne/git/dot-dagger/cmd/dotd/*.go
```

Expected: exactly one result — `prompts.go`.

- [ ] **Step 10: Run all tests**

```bash
cd /home/rocne/git/dot-dagger && go test ./cmd/dotd/...
```

Expected: all pass. `TestAnnotate_NonTTY` will now FAIL because the TTY guard no longer exists — this is expected and will be fixed in Task 5.

- [ ] **Step 11: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/annotate_cmd.go cmd/dotd/filter_prompt.go
git commit -m "refactor(annotate): remove direct huh usage, route all I/O through prompts helpers; remove isTTYStdin bridge"
```

---

## Task 5: Update integration tests for the annotate wizard

**Files:**
- Modify: `cmd/dotd/integration_test.go`

Replace the old non-TTY error test with tests that drive the actual wizard via piped stdin. The fixture file is `testdata/dotfiles/shellrc/aliases.sh`, which starts with:
```
#!/bin/bash
# @after(shellrc.path)
alias ll='ls -la'
```

In huh accessible mode, `NewSelect` renders a numbered list and reads an integer. Registry order: 1=When, 2=After, 3=Require, 4=Request, 5=Action, 6=Name, 7=Disable, 8=Done. `NewInput` reads a line. `NewConfirm` reads "y" or "n".

- [ ] **Step 1: Remove `TestAnnotate_NonTTY`**

Find and delete the entire `TestAnnotate_NonTTY` function from `cmd/dotd/integration_test.go`.

- [ ] **Step 2: Add the four new wizard tests**

Add the following tests at the bottom of `cmd/dotd/integration_test.go`:

```go
// TestAnnotate_AddWhen drives the wizard to add a @when(os=macos) annotation.
// Accessible-mode input: select When (1), enter value, select Done (8), confirm Yes.
func TestAnnotate_AddWhen(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	// aliases.sh already has @after(shellrc.path). We're adding @when on top.
	// Menu: 8 options (7 types + Done). Select 1=When, input value, 8=Done, y=Yes.
	stdin := strings.NewReader("1\nos=macos\n8\ny\n")
	_, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v", err)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	got := string(content)
	if !strings.Contains(got, "# @when(os=macos)") {
		t.Errorf("expected @when(os=macos) in file, got:\n%s", got)
	}
	if !strings.Contains(got, "# @after(shellrc.path)") {
		t.Errorf("expected original @after(shellrc.path) preserved, got:\n%s", got)
	}
}

// TestAnnotate_SetAction drives the wizard to set @action(source).
// Accessible-mode input: select Action (5), select source (1 of [source,no-source,link,none]), Done (8), Yes.
func TestAnnotate_SetAction(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	stdin := strings.NewReader("5\n1\n8\ny\n")
	_, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v", err)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), "# @action(source)") {
		t.Errorf("expected @action(source) in file, got:\n%s", string(content))
	}
}

// TestAnnotate_SetDisable drives the wizard to set @disable.
// Accessible-mode input: select Disable (7), confirm Set (y), Done (8), confirm write (y).
func TestAnnotate_SetDisable(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	stdin := strings.NewReader("7\ny\n8\ny\n")
	_, err := e.runWithStdin(t, stdin, "annotate", target)
	if err != nil {
		t.Fatalf("annotate wizard failed: %v", err)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(content), "# @disable") {
		t.Errorf("expected @disable in file, got:\n%s", string(content))
	}
}

// TestAnnotate_CancelAtConfirm drives the wizard to Done with no changes,
// then cancels at the final confirm. The file must be unmodified.
func TestAnnotate_CancelAtConfirm(t *testing.T) {
	e := newIenv(t)
	target := filepath.Join(e.dotfiles, "shellrc", "aliases.sh")

	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	// Done immediately (8), then No at confirm (n).
	stdin := strings.NewReader("8\nn\n")
	_, runErr := e.runWithStdin(t, stdin, "annotate", target)
	if runErr != nil {
		t.Fatalf("annotate wizard failed: %v", runErr)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("file changed after cancel; before:\n%s\nafter:\n%s", before, after)
	}
}
```

- [ ] **Step 3: Verify the tests compile**

```bash
cd /home/rocne/git/dot-dagger && go build ./cmd/dotd/...
```

- [ ] **Step 4: Run the integration tests — verify they pass**

```bash
cd /home/rocne/git/dot-dagger && go test -tags integration ./cmd/dotd/... -run 'TestAnnotate' -v
```

Expected:
```
--- PASS: TestAnnotate_AddWhen
--- PASS: TestAnnotate_SetAction
--- PASS: TestAnnotate_SetDisable
--- PASS: TestAnnotate_CancelAtConfirm
```

If a test fails with unexpected input/output mismatch, the accessible mode rendering may differ from expectations. Debug by printing the wizard output (`t.Logf("%s", out)` from `runWithStdin`) and counting the option numbers from the actual rendered menu.

- [ ] **Step 5: Run the full test suite**

```bash
cd /home/rocne/git/dot-dagger && go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add cmd/dotd/integration_test.go
git commit -m "test(annotate): replace non-TTY error test with 4 wizard integration tests"
```

---

## Task 6: Add Docker E2E test for `annotate`

**Files:**
- Create: `test/e2e/annotate.sh`
- Modify: `test/e2e/Dockerfile`
- Modify: `test/e2e/Dockerfile.local`
- Modify: `test/run-e2e.sh`

The e2e fixture lives at `/fixture` inside Docker. `fixture/shellrc/aliases.sh` has `# @after(shellrc.path)`.

- [ ] **Step 1: Create `test/e2e/annotate.sh`**

```sh
#!/bin/sh
set -e

# Work on a copy so we don't mutate the read-only /fixture mount.
cp -r /fixture /tmp/dotfiles

TARGET=/tmp/dotfiles/shellrc/aliases.sh

# ── Test 1: add @when(os=linux) ─────────────────────────────────────────────
# Accessible-mode input: select When (1), enter value, Done (8), confirm Yes (y).
printf '1\nos=linux\n8\ny\n' | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

grep -q '@when(os=linux)' "$TARGET" \
  || { printf 'FAIL: @when(os=linux) not written\n'; exit 1; }

grep -q '@after(shellrc.path)' "$TARGET" \
  || { printf 'FAIL: original @after annotation not preserved\n'; exit 1; }

printf 'PASS: annotate add @when\n'

# ── Test 2: set @action(source) ─────────────────────────────────────────────
# Select Action (5), select source (1), Done (8), Yes (y).
printf '5\n1\n8\ny\n' | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

grep -q '@action(source)' "$TARGET" \
  || { printf 'FAIL: @action(source) not written\n'; exit 1; }

printf 'PASS: annotate set @action\n'

# ── Test 3: cancel leaves file unchanged ────────────────────────────────────
cp "$TARGET" /tmp/aliases_before.sh

# Done immediately (8), No at confirm (n).
printf '8\nn\n' | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

diff -q "$TARGET" /tmp/aliases_before.sh \
  || { printf 'FAIL: file changed after cancel\n'; exit 1; }

printf 'PASS: annotate cancel unchanged\n'
printf 'PASS: annotate e2e\n'
```

- [ ] **Step 2: Add `annotate.sh` to `test/e2e/Dockerfile`**

Find the last `COPY` line in `test/e2e/Dockerfile` (currently `COPY procure/ /procure/`) and insert before it:

```dockerfile
COPY annotate.sh /tests/annotate.sh
```

- [ ] **Step 3: Add `annotate.sh` to `test/e2e/Dockerfile.local`**

Same change in `test/e2e/Dockerfile.local` — insert before the final `COPY dotd /staged/dotd` line:

```dockerfile
COPY annotate.sh /tests/annotate.sh
```

- [ ] **Step 4: Add `annotate.sh` to `test/run-e2e.sh`**

Find the last `run_test` call (currently `run_test package-check.sh`) and add after it:

```sh
run_test annotate.sh
```

- [ ] **Step 5: Run the full e2e suite locally**

```bash
cd /home/rocne/git/dot-dagger && ./test/run-e2e.sh
```

Expected: all tests pass including `=== annotate.sh ===` with three `PASS` lines.

If `annotate.sh` fails, the most likely cause is option number mismatch (the menu numbering in Docker vs the test assumptions). Run `printf '8\nn\n' | dotd annotate --files /tmp/dotfiles --env-file /tmp/dotfiles/env.yaml /tmp/dotfiles/shellrc/aliases.sh` interactively in Docker to see the actual accessible-mode output, then correct the option numbers.

- [ ] **Step 6: Run full unit + integration test suite**

```bash
cd /home/rocne/git/dot-dagger && go test ./... && go test -tags integration ./cmd/dotd/...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
cd /home/rocne/git/dot-dagger
git add test/e2e/annotate.sh test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh
git commit -m "test(e2e): add annotate Docker e2e tests"
```

---

## Self-Review

### Spec coverage check

| Requirement | Task |
|-------------|------|
| `prompts.go` is the only file importing huh | Task 4, Step 9 (grep verification) |
| `isTTY(io.Reader)` replaces `isTTYStdin()` | Task 1 (add), Task 2 (bridge shim), Task 4 (final removal) |
| All huh calls route through cobra I/O | Task 1 (`newHuhForm` helper) |
| `filter_prompt.go` loses huh import | Task 2 |
| `adopt.go` loses huh import | Task 3 |
| `isTTYStdin` bridge shim removed | Task 4, Step 2 |
| `annotate_cmd.go` loses huh import | Task 4 |
| `printPersistHint(os.Stderr)` fixed | Task 2 |
| `prompter` test seam removed | Task 2 |
| `errUserAborted` moves to `prompts.go` | Task 1 |
| Integration tests drive wizard via stdin | Task 5 |
| Docker e2e tests | Task 6 |

### Type consistency

- `promptMenu(cmd *cobra.Command, title string, options []string) (int, error)` — defined Task 1, used Task 4
- `promptSelect(cmd, title, desc string, options []string) (string, error)` — defined Task 1, used Task 4
- `promptInput(cmd, title, desc, prefill string, validate func(string) error) (string, error)` — defined Task 1, used Task 4
- `promptBool(cmd, title, desc, affirm, neg string, initial bool) (bool, error)` — defined Task 1, used Tasks 3 and 4
- `promptInputs(cmd *cobra.Command, prompts []inputPrompt) (map[string]string, error)` — defined Task 1, used Task 2
- `filterWithPrompt(cmd *cobra.Command, nodes []pipeline.RawNode, resolved map[string]string, tty bool)` — defined Task 2, call sites updated Task 2
- `isTTY(r io.Reader) bool` — defined Task 1, used Tasks 2 and 3

### Known limitation

In huh accessible mode, `promptInput` uses the `prefill` value as the default — pressing Enter in a test keeps the existing value rather than clearing it. This means "remove by clearing" only works in full TUI mode. The integration tests test the add-new-value path (no pre-existing value). Clearing an existing text annotation requires a manual TTY test (listed in the annotate wizard's Self-Review Checklist).
