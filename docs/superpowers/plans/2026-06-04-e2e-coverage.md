# E2E Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add complete e2e test coverage (shell e2e + Go integration) for all dotd commands that currently have gaps.

**Architecture:** Five independently-mergeable batches, each adding shell e2e scripts under `test/e2e/` and Go integration tests in `cmd/dotd/integration_test.go`. Shell e2e tests run real binaries in Docker via `run_test`; integration tests run in-process with `//go:build integration`. Both layers cover the same gap.

**Tech Stack:** Go testing, shell (POSIX sh), Docker (Ubuntu 24.04), cobra `cmd.SetIn`/`cmd.SetOut` for piped stdin in integration tests.

---

## Reference

### Integration test runner

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestName -v
go test -tags integration -count=1 ./cmd/dotd/ -v   # all integration tests
```

### Shell e2e runner

```bash
./test/run-e2e.sh
```

### ienv base flags (auto-added by `e.run` / `e.runMayFail` / `e.runWithStdin`)

```
--files <e.dotfiles>
--env-file <e.dotfiles>/env.yaml
--link-root <e.home>
--bin-dir <e.binDir>
--init-file <e.initFile>
--generated-dir <e.generatedDir>
```

---

## Task 1: Add `runWithStdin` + unapply integration tests

**Files:** Modify `cmd/dotd/integration_test.go`

- [ ] **Step 1.1: Add `"io"` to imports**

Open `cmd/dotd/integration_test.go`. Change the import block from:

```go
import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

to:

```go
import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 1.2: Add `runWithStdin` method to `ienv`**

Add immediately after the `runMayFail` method (after line 88):

```go
// runWithStdin executes a dotd subcommand with piped stdin. Returns output and
// error without failing the test — callers decide how to handle the result.
func (e *ienv) runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	base := []string{
		"--files", e.dotfiles,
		"--env-file", filepath.Join(e.dotfiles, "env.yaml"),
		"--link-root", e.home,
		"--bin-dir", e.binDir,
		"--init-file", e.initFile,
		"--generated-dir", e.generatedDir,
	}
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetIn(stdin)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append(base, args...))
	return buf.String(), cmd.Execute()
}
```

- [ ] **Step 1.3: Run existing integration tests to confirm no compilation errors**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -v 2>&1 | tail -20
```

Expected: all existing tests PASS, no compilation error.

- [ ] **Step 1.4: Add `TestUnapplyAfterApply`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestUnapplyAfterApply verifies that unapply --yes removes all symlinks and
// init.sh created by apply.
func TestUnapplyAfterApply(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	// Confirm apply produced symlinks and init.sh.
	assertSymlink(t,
		filepath.Join(e.home, ".zshrc"),
		filepath.Join(e.dotfiles, "config/dot-zshrc"),
	)
	if _, err := os.Stat(e.initFile); err != nil {
		t.Fatalf("init.sh not created by apply: %v", err)
	}

	// Unapply.
	e.run(t, "unapply", "--yes", "--env", "os=linux", "--env", "context=personal")

	// All symlinks removed.
	assertNoPath(t, filepath.Join(e.home, ".zshrc"))
	assertNoPath(t, filepath.Join(e.home, ".gitconfig"))
	assertNoPath(t, filepath.Join(e.binDir, "hello"))
	// init.sh removed.
	assertNoPath(t, e.initFile)
}
```

- [ ] **Step 1.5: Run `TestUnapplyAfterApply` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestUnapplyAfterApply -v
```

Expected: `--- PASS: TestUnapplyAfterApply`

- [ ] **Step 1.6: Add `TestUnapplyCancel`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestUnapplyCancel verifies that answering "n" to the confirmation prompt
// exits 0 and preserves all symlinks.
func TestUnapplyCancel(t *testing.T) {
	e := newIenv(t)
	e.run(t, "apply", "--env", "os=linux", "--env", "context=personal")

	zshrc := filepath.Join(e.home, ".zshrc")
	assertSymlink(t, zshrc, filepath.Join(e.dotfiles, "config/dot-zshrc"))

	// Pipe "n\n" to unapply — should cancel without removing anything.
	out, err := e.runWithStdin(t, strings.NewReader("n\n"),
		"unapply", "--env", "os=linux", "--env", "context=personal")
	if err != nil {
		t.Fatalf("unapply cancel: %v\noutput: %s", err, out)
	}

	// Symlink preserved.
	assertSymlink(t, zshrc, filepath.Join(e.dotfiles, "config/dot-zshrc"))
	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output: %q", out)
	}
}
```

- [ ] **Step 1.7: Run `TestUnapplyCancel` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestUnapplyCancel -v
```

Expected: `--- PASS: TestUnapplyCancel`

- [ ] **Step 1.8: Commit integration tests**

```bash
git add cmd/dotd/integration_test.go
git commit -m "test(integration): add TestUnapplyAfterApply and TestUnapplyCancel with runWithStdin harness"
```

---

## Task 2: Add unapply shell e2e scripts

**Files:** Create `test/e2e/unapply.sh`, `test/e2e/unapply-cancel.sh`

- [ ] **Step 2.1: Create `test/e2e/unapply.sh`**

```bash
cat > test/e2e/unapply.sh << 'EOF'
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

dotd unapply --yes \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test ! -L /home/e2e/.zshrc        || { printf 'FAIL: .zshrc symlink should be removed\n'; exit 1; }
test ! -f /tmp/init.sh             || { printf 'FAIL: init.sh should be removed\n'; exit 1; }
test ! -L /home/e2e/bin/hello      || { printf 'FAIL: bin/hello symlink should be removed\n'; exit 1; }

printf 'PASS: unapply test\n'
EOF
chmod +x test/e2e/unapply.sh
```

- [ ] **Step 2.2: Create `test/e2e/unapply-cancel.sh`**

```bash
cat > test/e2e/unapply-cancel.sh << 'EOF'
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

printf 'n\n' | dotd unapply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test -L /home/e2e/.zshrc || { printf 'FAIL: .zshrc symlink should be preserved on cancel\n'; exit 1; }

printf 'PASS: unapply-cancel test\n'
EOF
chmod +x test/e2e/unapply-cancel.sh
```

- [ ] **Step 2.3: Commit shell e2e scripts**

```bash
git add test/e2e/unapply.sh test/e2e/unapply-cancel.sh
git commit -m "test(e2e): add unapply.sh and unapply-cancel.sh shell e2e tests"
```

---

## Task 3: Wire Batch 1 into Dockerfiles and runners, then push PR

**Files:** Modify `test/e2e/Dockerfile`, `test/e2e/Dockerfile.local`, `test/run-e2e.sh`, `test/run-e2e-release.sh`

- [ ] **Step 3.1: Add COPY lines to `test/e2e/Dockerfile`**

After the existing `COPY conflict.sh /tests/conflict.sh` line, add:

```
COPY unapply.sh /tests/unapply.sh
COPY unapply-cancel.sh /tests/unapply-cancel.sh
```

- [ ] **Step 3.2: Add COPY lines to `test/e2e/Dockerfile.local`**

Same two COPY lines, in the same position (before the `COPY dotd /staged/dotd` line).

- [ ] **Step 3.3: Add `run_test` calls to `test/run-e2e.sh`**

After the existing `run_test conflict.sh` line, add:

```sh
run_test unapply.sh
run_test unapply-cancel.sh
```

- [ ] **Step 3.4: Add `run_test` calls to `test/run-e2e-release.sh`**

After the existing `run_test conflict.sh` line, add:

```sh
run_test unapply.sh
run_test unapply-cancel.sh
```

- [ ] **Step 3.5: Run the full e2e suite locally to confirm both new scripts pass**

```bash
./test/run-e2e.sh
```

Expected: `=== unapply.sh ===` ... `PASS: unapply test` and `=== unapply-cancel.sh ===` ... `PASS: unapply-cancel test`; `All e2e tests passed.`

- [ ] **Step 3.6: Commit and push**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh test/run-e2e-release.sh
git commit -m "test(e2e): wire unapply tests into Dockerfiles and runners (Batch 1)"
git push
gh pr view   # confirm PR is open before pushing
```

---

## Task 4: Extend e2e fixture for compose (Batch 2 infrastructure)

**Files:** Create `test/e2e/fixture/shellrc/dot-extras.sh.d/.dagger`, `test/e2e/fixture/shellrc/dot-extras.sh.d/base.sh`, `test/e2e/fixture/shellrc/dot-extras.sh.d/nosync-work.sh`

The e2e fixture does not have a compose target. We add a shell-scripts compose target. `dot-extras.sh.d/` strips `dot-` and `.d` → generates `extras.sh` in `--generated-dir`. The `base.sh` fragment is always active; `nosync-work.sh` is gated on `context=work`.

- [ ] **Step 4.1: Create compose target directory and `.dagger` file**

```bash
mkdir -p test/e2e/fixture/shellrc/dot-extras.sh.d
```

Create `test/e2e/fixture/shellrc/dot-extras.sh.d/.dagger`:

```yaml
composition:
  enabled: true
actions:
  - source
```

- [ ] **Step 4.2: Create `base.sh` fragment**

Create `test/e2e/fixture/shellrc/dot-extras.sh.d/base.sh`:

```sh
#!/bin/sh
export EXTRAS_BASE=1
```

- [ ] **Step 4.3: Create `nosync-work.sh` fragment (work-gated)**

Create `test/e2e/fixture/shellrc/dot-extras.sh.d/nosync-work.sh`:

```sh
#!/bin/sh
# @when(context=work)
export EXTRAS_WORK=1
```

- [ ] **Step 4.4: Confirm existing integration tests still pass (fixture is shared with e2e, not testdata)**

The testdata fixture used by integration tests (`cmd/dotd/testdata/dotfiles/`) is separate from the e2e fixture. No integration test changes needed.

```bash
go test -tags integration -count=1 ./cmd/dotd/ -v 2>&1 | tail -5
```

Expected: all tests PASS.

- [ ] **Step 4.5: Commit fixture extension**

```bash
git add test/e2e/fixture/shellrc/dot-extras.sh.d/
git commit -m "test(e2e/fixture): add dot-extras.sh.d compose target for Batch 2 tests"
```

---

## Task 5: Add compose + macOS shell e2e scripts (Batch 2)

**Files:** Create `test/e2e/compose.sh`, `test/e2e/macos-apply.sh`

- [ ] **Step 5.1: Create `test/e2e/compose.sh`**

```bash
cat > test/e2e/compose.sh << 'EOF'
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test -f /tmp/generated/extras.sh || { printf 'FAIL: extras.sh not generated\n'; exit 1; }
grep -q "extras.sh" /tmp/init.sh || { printf 'FAIL: extras.sh not sourced in init.sh\n'; exit 1; }

OUT=$(dotd compose list \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal)
printf '%s' "$OUT" | grep -q "extras.sh" \
  || { printf 'FAIL: compose list missing extras.sh\n'; exit 1; }

dotd compose check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal \
  || { printf 'FAIL: compose check should pass after apply\n'; exit 1; }

printf 'stale content\n' > /tmp/generated/extras.sh

OUT=$(dotd compose check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal)
printf '%s' "$OUT" | grep -q "stale" \
  || { printf 'FAIL: compose check should report stale\n'; exit 1; }

printf 'PASS: compose test\n'
EOF
chmod +x test/e2e/compose.sh
```

- [ ] **Step 5.2: Create `test/e2e/macos-apply.sh`**

```bash
cat > test/e2e/macos-apply.sh << 'EOF'
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=macos \
  --env context=personal

grep -q "macos.sh"   /tmp/init.sh || { printf 'FAIL: macos.sh not in init.sh\n'; exit 1; }
! grep -q "linux.sh" /tmp/init.sh || { printf 'FAIL: linux.sh should not be in init.sh for os=macos\n'; exit 1; }

printf 'PASS: macos-apply test\n'
EOF
chmod +x test/e2e/macos-apply.sh
```

- [ ] **Step 5.3: Commit shell e2e scripts**

```bash
git add test/e2e/compose.sh test/e2e/macos-apply.sh
git commit -m "test(e2e): add compose.sh and macos-apply.sh shell e2e tests"
```

---

## Task 6: Wire Batch 2 into Dockerfiles and runners, then run and push

**Files:** Modify `test/e2e/Dockerfile`, `test/e2e/Dockerfile.local`, `test/run-e2e.sh`, `test/run-e2e-release.sh`

- [ ] **Step 6.1: Add COPY lines to both Dockerfiles**

In both `test/e2e/Dockerfile` and `test/e2e/Dockerfile.local`, after the Batch 1 COPY lines, add:

```
COPY compose.sh /tests/compose.sh
COPY macos-apply.sh /tests/macos-apply.sh
```

- [ ] **Step 6.2: Add `run_test` calls to both runners**

In both `test/run-e2e.sh` and `test/run-e2e-release.sh`, after the Batch 1 `run_test` lines, add:

```sh
run_test compose.sh
run_test macos-apply.sh
```

- [ ] **Step 6.3: Run the full e2e suite**

```bash
./test/run-e2e.sh
```

Expected: `PASS: compose test` and `PASS: macos-apply test` appear; `All e2e tests passed.`

- [ ] **Step 6.4: Commit, verify PR open, push**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh test/run-e2e-release.sh
git commit -m "test(e2e): wire compose and macos-apply tests into Dockerfiles and runners (Batch 2)"
gh pr view
git push
```

---

## Task 7: Add setup/init/teardown integration tests (Batch 3)

**Files:** Modify `cmd/dotd/integration_test.go`

These tests call `newRootCmd()` directly (not via `ienv`) because they manage `XDG_CONFIG_HOME` and `DOTFILES` env vars.

- [ ] **Step 7.1: Add `TestSetupThenTeardown`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestSetupThenTeardown verifies the full setup → teardown lifecycle:
//   - setup writes config.yaml to the XDG config dir
//   - teardown with --yes removes it
func TestSetupThenTeardown(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	// Run setup: accept all prompts with Enter (use defaults).
	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(strings.Repeat("\n", 10)))
	var setupBuf bytes.Buffer
	setupCmd.SetOut(&setupBuf)
	setupCmd.SetErr(&setupBuf)
	setupCmd.SetArgs([]string{"setup"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, setupBuf.String())
	}

	configPath := filepath.Join(xdg, "dot-dagger", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not written by setup: %v", err)
	}

	// Run teardown: confirm with "y". Use emptyEnvFile to avoid shell-expansion
	// issues in env.yaml written by setup.
	teardownCmd := newRootCmd()
	teardownCmd.SetIn(strings.NewReader("y\n"))
	var teardownBuf bytes.Buffer
	teardownCmd.SetOut(&teardownBuf)
	teardownCmd.SetErr(&teardownBuf)
	teardownCmd.SetArgs([]string{"teardown",
		"--files", emptyDotfiles(t),
		"--env-file", emptyEnvFile(t),
	})
	if err := teardownCmd.Execute(); err != nil {
		t.Fatalf("teardown error = %v\noutput:\n%s", err, teardownBuf.String())
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config.yaml should be removed by teardown")
	}
}
```

- [ ] **Step 7.2: Run `TestSetupThenTeardown` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestSetupThenTeardown -v
```

Expected: `--- PASS: TestSetupThenTeardown`

- [ ] **Step 7.3: Add `TestInitAfterSetup`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestInitAfterSetup verifies that init scaffolds .dagger in shellrc/, config/,
// and bin/ after setup has written config.yaml.
func TestInitAfterSetup(t *testing.T) {
	xdg := t.TempDir()
	dotfilesDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("DOTFILES", dotfilesDir)

	// Run setup first (accept all defaults).
	setupCmd := newRootCmd()
	setupCmd.SetIn(strings.NewReader(strings.Repeat("\n", 10)))
	var setupBuf bytes.Buffer
	setupCmd.SetOut(&setupBuf)
	setupCmd.SetErr(&setupBuf)
	setupCmd.SetArgs([]string{"setup"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("setup error = %v\noutput:\n%s", err, setupBuf.String())
	}

	// Run init:
	//   "y\n"  → create shellrc dir
	//   "\n"   → accept default name "shellrc"
	//   "y\n"  → create config dir
	//   "\n"   → accept default name "config"
	//   "y\n"  → create bin dir
	//   "\n"   → accept default name "bin"
	//   "n\n"  → explicit "no" to source-line prompt (avoids touching real RC files)
	initCmd := newRootCmd()
	initCmd.SetIn(strings.NewReader("y\n\ny\n\ny\n\nn\n"))
	var initBuf bytes.Buffer
	initCmd.SetOut(&initBuf)
	initCmd.SetErr(&initBuf)
	initCmd.SetArgs([]string{"init",
		"--files", dotfilesDir,
		"--env-file", emptyEnvFile(t),
	})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init error = %v\noutput:\n%s", err, initBuf.String())
	}

	for _, dir := range []string{"shellrc", "config", "bin"} {
		daggerPath := filepath.Join(dotfilesDir, dir, ".dagger")
		if _, err := os.Stat(daggerPath); err != nil {
			t.Errorf(".dagger not scaffolded in %s/: %v", dir, err)
		}
	}
}
```

- [ ] **Step 7.4: Run `TestInitAfterSetup` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestInitAfterSetup -v
```

Expected: `--- PASS: TestInitAfterSetup`

- [ ] **Step 7.5: Commit integration tests**

```bash
git add cmd/dotd/integration_test.go
git commit -m "test(integration): add TestSetupThenTeardown and TestInitAfterSetup"
```

---

## Task 8: Add setup/init/teardown shell e2e scripts (Batch 3)

**Files:** Create `test/e2e/setup.sh`, `test/e2e/teardown-confirm.sh`, `test/e2e/teardown-cancel.sh`, `test/e2e/init.sh`

Each Docker container run is fresh. `/tmp` is empty at the start of each script.

- [ ] **Step 8.1: Create `test/e2e/setup.sh`**

```bash
cat > test/e2e/setup.sh << 'EOF'
#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
export DOTFILES=/fixture

printf '\n\n\n\n' | dotd setup

test -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml not written\n'; exit 1; }
grep -q "dotfiles: /fixture" /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: dotfiles not set to /fixture in config.yaml\n'; exit 1; }

printf 'PASS: setup test\n'
EOF
chmod +x test/e2e/setup.sh
```

- [ ] **Step 8.2: Create `test/e2e/teardown-confirm.sh`**

```bash
cat > test/e2e/teardown-confirm.sh << 'EOF'
#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml

printf 'y\n' | dotd teardown \
  --files /fixture \
  --env-file /fixture/env.yaml

test ! -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be removed after teardown\n'; exit 1; }

printf 'PASS: teardown-confirm test\n'
EOF
chmod +x test/e2e/teardown-confirm.sh
```

- [ ] **Step 8.3: Create `test/e2e/teardown-cancel.sh`**

```bash
cat > test/e2e/teardown-cancel.sh << 'EOF'
#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml

printf 'n\n' | dotd teardown \
  --files /fixture \
  --env-file /fixture/env.yaml

test -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be preserved on cancel\n'; exit 1; }

printf 'PASS: teardown-cancel test\n'
EOF
chmod +x test/e2e/teardown-cancel.sh
```

- [ ] **Step 8.4: Create `test/e2e/init.sh`**

```bash
cat > test/e2e/init.sh << 'EOF'
#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /tmp/testdotfiles\n' > /tmp/xdg/dot-dagger/config.yaml
mkdir -p /tmp/testdotfiles

# Prompts: 3 × (y = create dir, Enter = accept default name) + trailing EOF → no source line
printf 'y\n\ny\n\ny\n\n' | dotd init --config /tmp/xdg/dot-dagger/config.yaml

test -f /tmp/testdotfiles/shellrc/.dagger \
  || { printf 'FAIL: shellrc/.dagger not scaffolded\n'; exit 1; }
test -f /tmp/testdotfiles/config/.dagger \
  || { printf 'FAIL: config/.dagger not scaffolded\n'; exit 1; }
test -f /tmp/testdotfiles/bin/.dagger \
  || { printf 'FAIL: bin/.dagger not scaffolded\n'; exit 1; }

printf 'PASS: init test\n'
EOF
chmod +x test/e2e/init.sh
```

- [ ] **Step 8.5: Commit shell e2e scripts**

```bash
git add test/e2e/setup.sh test/e2e/teardown-confirm.sh test/e2e/teardown-cancel.sh test/e2e/init.sh
git commit -m "test(e2e): add setup, teardown-confirm, teardown-cancel, init shell e2e tests"
```

---

## Task 9: Wire Batch 3 into Dockerfiles and runners, then run and push

**Files:** Modify `test/e2e/Dockerfile`, `test/e2e/Dockerfile.local`, `test/run-e2e.sh`, `test/run-e2e-release.sh`

- [ ] **Step 9.1: Add COPY lines to both Dockerfiles**

After the Batch 2 COPY lines, add:

```
COPY setup.sh /tests/setup.sh
COPY teardown-confirm.sh /tests/teardown-confirm.sh
COPY teardown-cancel.sh /tests/teardown-cancel.sh
COPY init.sh /tests/init.sh
```

- [ ] **Step 9.2: Add `run_test` calls to both runners**

After the Batch 2 `run_test` lines, add:

```sh
run_test setup.sh
run_test teardown-confirm.sh
run_test teardown-cancel.sh
run_test init.sh
```

- [ ] **Step 9.3: Run the full e2e suite**

```bash
./test/run-e2e.sh
```

Expected: `PASS: setup test`, `PASS: teardown-confirm test`, `PASS: teardown-cancel test`, `PASS: init test` all appear; `All e2e tests passed.`

- [ ] **Step 9.4: Commit, verify PR open, push**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh test/run-e2e-release.sh
git commit -m "test(e2e): wire setup/init/teardown tests into Dockerfiles and runners (Batch 3)"
gh pr view
git push
```

---

## Task 10: Add config/env integration tests (Batch 4)

**Files:** Modify `cmd/dotd/integration_test.go`

`TestConfigCmdsLifecycle` uses `run()` and `writeConfigYAML()` from `main_test.go`/`config_cmd_test.go` — available since they share `package main` with no build tag restriction.
`TestEnvCmdsLifecycle` uses `ienv` since it needs the testdata fixture's env.yaml.

- [ ] **Step 10.1: Add `TestConfigCmdsLifecycle`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestConfigCmdsLifecycle verifies the config show → set → get round-trip via the
// full cobra command path (PersistentPreRunE, resolvePaths, etc.).
func TestConfigCmdsLifecycle(t *testing.T) {
	// config show on a missing config file exits 0 with empty values.
	missingConfig := filepath.Join(t.TempDir(), "config.yaml")
	out, err := run(t, "config", "show",
		"--config", missingConfig,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config show on missing file: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "dotfiles=") {
		t.Errorf("config show: expected dotfiles= in output: %q", out)
	}

	// config set dotfiles /tmp/x → config get dotfiles returns /tmp/x
	configPath := writeConfigYAML(t, "{}\n")
	if _, err = run(t, "config", "set", "dotfiles", "/tmp/x",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	); err != nil {
		t.Fatalf("config set error = %v", err)
	}

	out, err = run(t, "config", "get", "dotfiles",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get error = %v", err)
	}
	if strings.TrimSpace(out) != "/tmp/x" {
		t.Errorf("config get dotfiles = %q, want /tmp/x", strings.TrimSpace(out))
	}
}
```

- [ ] **Step 10.2: Run `TestConfigCmdsLifecycle` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestConfigCmdsLifecycle -v
```

Expected: `--- PASS: TestConfigCmdsLifecycle`

- [ ] **Step 10.3: Add `TestEnvCmdsLifecycle`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestEnvCmdsLifecycle verifies env show → set → get → diff via the testdata fixture.
func TestEnvCmdsLifecycle(t *testing.T) {
	e := newIenv(t)

	// env show: testdata env.yaml has context=personal
	out := e.run(t, "env", "show")
	if !strings.Contains(out, "context=personal") {
		t.Errorf("env show: expected context=personal: %q", out)
	}

	// env set context → staging
	e.run(t, "env", "set", "context", "staging")

	// env get context → staging
	out = e.run(t, "env", "get", "context")
	if strings.TrimSpace(out) != "staging" {
		t.Errorf("env get context = %q, want staging", strings.TrimSpace(out))
	}

	// env diff: env.yaml has context=staging; DOTD_CONTEXT not set → shows override.
	// --env os=macos is ignored by env diff (it shows file-vs-shell diffs only),
	// but must not cause an error.
	out = e.run(t, "env", "diff", "--env", "os=macos")
	if !strings.Contains(out, "context") {
		t.Errorf("env diff: expected 'context' in output: %q", out)
	}
}
```

- [ ] **Step 10.4: Run `TestEnvCmdsLifecycle` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestEnvCmdsLifecycle -v
```

Expected: `--- PASS: TestEnvCmdsLifecycle`

- [ ] **Step 10.5: Commit integration tests**

```bash
git add cmd/dotd/integration_test.go
git commit -m "test(integration): add TestConfigCmdsLifecycle and TestEnvCmdsLifecycle"
```

---

## Task 11: Add config/env shell e2e scripts (Batch 4)

**Files:** Create `test/e2e/config-cmds.sh`, `test/e2e/env-cmds.sh`

- [ ] **Step 11.1: Create `test/e2e/config-cmds.sh`**

```bash
cat > test/e2e/config-cmds.sh << 'EOF'
#!/bin/sh
set -e

printf 'dotfiles: /fixture\n' > /tmp/dotd.yaml

dotd config show \
  --config /tmp/dotd.yaml \
  --env-file /fixture/env.yaml \
  --files /fixture \
  | grep -q "dotfiles" || { printf 'FAIL: config show missing dotfiles\n'; exit 1; }

VAL=$(dotd config get dotfiles \
  --config /tmp/dotd.yaml \
  --env-file /fixture/env.yaml \
  --files /fixture)
[ "$VAL" = "/fixture" ] \
  || { printf 'FAIL: config get dotfiles = %s, want /fixture\n' "$VAL"; exit 1; }

dotd config set dotfiles /fixture2 \
  --config /tmp/dotd.yaml \
  --env-file /fixture/env.yaml \
  --files /fixture

VAL=$(dotd config get dotfiles \
  --config /tmp/dotd.yaml \
  --env-file /fixture/env.yaml \
  --files /fixture)
[ "$VAL" = "/fixture2" ] \
  || { printf 'FAIL: config get after set = %s, want /fixture2\n' "$VAL"; exit 1; }

printf 'PASS: config-cmds test\n'
EOF
chmod +x test/e2e/config-cmds.sh
```

- [ ] **Step 11.2: Create `test/e2e/env-cmds.sh`**

The fixture is read-only (`-v fixture:/fixture:ro`). Copy env.yaml to `/tmp` so `env set` can write to it.

```bash
cat > test/e2e/env-cmds.sh << 'EOF'
#!/bin/sh
set -e

# fixture is read-only; copy env.yaml so env set can write to it
cp /fixture/env.yaml /tmp/env.yaml

OUT=$(dotd env show \
  --env-file /tmp/env.yaml \
  --files /fixture)
printf '%s' "$OUT" | grep -q "context" \
  || { printf 'FAIL: env show missing context\n'; exit 1; }

VAL=$(dotd env get context \
  --env-file /tmp/env.yaml \
  --files /fixture)
[ "$VAL" = "personal" ] \
  || { printf 'FAIL: env get context = %s, want personal\n' "$VAL"; exit 1; }

dotd env set context staging \
  --env-file /tmp/env.yaml \
  --files /fixture

VAL=$(dotd env get context \
  --env-file /tmp/env.yaml \
  --files /fixture)
[ "$VAL" = "staging" ] \
  || { printf 'FAIL: env get after set = %s, want staging\n' "$VAL"; exit 1; }

OUT=$(dotd env diff \
  --env-file /tmp/env.yaml \
  --files /fixture \
  --env os=macos)
printf '%s' "$OUT" | grep -q "context" \
  || { printf 'FAIL: env diff missing context\n'; exit 1; }

printf 'PASS: env-cmds test\n'
EOF
chmod +x test/e2e/env-cmds.sh
```

- [ ] **Step 11.3: Commit shell e2e scripts**

```bash
git add test/e2e/config-cmds.sh test/e2e/env-cmds.sh
git commit -m "test(e2e): add config-cmds.sh and env-cmds.sh shell e2e tests"
```

---

## Task 12: Wire Batch 4 into Dockerfiles and runners, then run and push

**Files:** Modify `test/e2e/Dockerfile`, `test/e2e/Dockerfile.local`, `test/run-e2e.sh`, `test/run-e2e-release.sh`

- [ ] **Step 12.1: Add COPY lines to both Dockerfiles**

After the Batch 3 COPY lines, add:

```
COPY config-cmds.sh /tests/config-cmds.sh
COPY env-cmds.sh /tests/env-cmds.sh
```

- [ ] **Step 12.2: Add `run_test` calls to both runners**

After the Batch 3 `run_test` lines, add:

```sh
run_test config-cmds.sh
run_test env-cmds.sh
```

- [ ] **Step 12.3: Run the full e2e suite**

```bash
./test/run-e2e.sh
```

Expected: `PASS: config-cmds test` and `PASS: env-cmds test` appear; `All e2e tests passed.`

- [ ] **Step 12.4: Commit, verify PR open, push**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh test/run-e2e-release.sh
git commit -m "test(e2e): wire config-cmds and env-cmds tests into Dockerfiles and runners (Batch 4)"
gh pr view
git push
```

---

## Task 13: Add adopt/bundle integration tests (Batch 5)

**Files:** Modify `cmd/dotd/integration_test.go`

`adopt` auto-skips the huh confirmation when `isTTYStdin()` is false (no TTY in tests).
`bundle` with `aliases.sh` → deps chain: aliases → path → base → output contains `DOT_BASE_LOADED` from base.sh.

- [ ] **Step 13.1: Add `TestAdoptShellScript_Integration`**

Append to `cmd/dotd/integration_test.go`:

```go
// TestAdoptShellScript_Integration verifies that adopt moves a .sh file into
// the dotfiles shellrc/ directory and removes the source file.
// No TTY in tests → isTTYStdin() is false → huh prompt auto-skipped.
func TestAdoptShellScript_Integration(t *testing.T) {
	e := newIenv(t)

	// Source file in a separate temp dir (outside dotfiles).
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "newscript.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatalf("write srcPath: %v", err)
	}

	// adopt moves srcPath to e.dotfiles/shellrc/newscript.sh.
	// "--to shellrc/" with trailing slash appends the filename.
	e.run(t, "adopt", srcPath, "--to", "shellrc/")

	// Source file moved (no longer exists at original path).
	assertNoPath(t, srcPath)

	// Destination exists inside dotfiles.
	dest := filepath.Join(e.dotfiles, "shellrc", "newscript.sh")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("adopted file not found at %s: %v", dest, err)
	}
}
```

- [ ] **Step 13.2: Run `TestAdoptShellScript_Integration` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestAdoptShellScript_Integration -v
```

Expected: `--- PASS: TestAdoptShellScript_Integration`

- [ ] **Step 13.3: Add `TestBundleOutput_Integration`**

`aliases.sh` in testdata depends on `path.sh` which depends on `base.sh`. Bundling `aliases.sh` inlines all deps. `base.sh` contains `export DOT_BASE_LOADED=1`.

Append to `cmd/dotd/integration_test.go`:

```go
// TestBundleOutput_Integration verifies that bundle shellrc/aliases.sh inlines
// all transitive dependencies. aliases→path→base, so DOT_BASE_LOADED from
// base.sh must appear in the output.
func TestBundleOutput_Integration(t *testing.T) {
	e := newIenv(t)

	out := e.run(t, "bundle", "shellrc/aliases.sh",
		"--env", "os=linux", "--env", "context=personal")

	if !strings.Contains(out, "DOT_BASE_LOADED") {
		t.Errorf("bundle: expected DOT_BASE_LOADED (from base.sh) in output: %q", out)
	}
}
```

- [ ] **Step 13.4: Run `TestBundleOutput_Integration` and confirm PASS**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -run TestBundleOutput_Integration -v
```

Expected: `--- PASS: TestBundleOutput_Integration`

- [ ] **Step 13.5: Commit integration tests**

```bash
git add cmd/dotd/integration_test.go
git commit -m "test(integration): add TestAdoptShellScript_Integration and TestBundleOutput_Integration"
```

---

## Task 14: Add adopt/bundle/dag-check/package-check shell e2e scripts (Batch 5)

**Files:** Create `test/e2e/adopt.sh`, `test/e2e/bundle.sh`, `test/e2e/dag-check.sh`, `test/e2e/package-check.sh`

- [ ] **Step 14.1: Create `test/e2e/adopt.sh`**

`adopt` auto-skips the huh prompt when stdin is not a TTY (Docker). No `--yes` flag needed.
`--to shellrc/` with trailing slash → appends filename → destination `shellrc/newscript.sh`.

```bash
cat > test/e2e/adopt.sh << 'EOF'
#!/bin/sh
set -e

# fixture is read-only; make a writable copy
cp -r /fixture /tmp/dotfiles

# source file outside dotfiles dir
printf '#!/bin/sh\necho hi\n' > /tmp/newscript.sh

# adopt moves /tmp/newscript.sh into /tmp/dotfiles/shellrc/
# no TTY in Docker → huh prompt auto-skipped, runs non-interactively
dotd adopt /tmp/newscript.sh \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  --to shellrc/

test -f /tmp/dotfiles/shellrc/newscript.sh \
  || { printf 'FAIL: newscript.sh not adopted into shellrc/\n'; exit 1; }
test ! -f /tmp/newscript.sh \
  || { printf 'FAIL: source file should be moved (not copied)\n'; exit 1; }

printf 'PASS: adopt test\n'
EOF
chmod +x test/e2e/adopt.sh
```

- [ ] **Step 14.2: Create `test/e2e/bundle.sh`**

`aliases.sh` depends on `path.sh` which depends on `base.sh`. Bundle output contains all deps' content inlined. `base.sh` has `export DOT_BASE_LOADED=1`.

```bash
cat > test/e2e/bundle.sh << 'EOF'
#!/bin/sh
set -e

OUT=$(dotd bundle shellrc/aliases.sh \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "DOT_BASE_LOADED" \
  || { printf 'FAIL: bundle should contain DOT_BASE_LOADED from base.sh\n'; exit 1; }

printf 'PASS: bundle test\n'
EOF
chmod +x test/e2e/bundle.sh
```

- [ ] **Step 14.3: Create `test/e2e/dag-check.sh`**

`dag check` writes numbered logical names to stdout (e.g., `  1  shellrc.base`). Check that `shellrc.base`, `shellrc.path`, `shellrc.aliases` appear and that base appears on a lower line number than path.

```bash
cat > test/e2e/dag-check.sh << 'EOF'
#!/bin/sh
set -e

OUT=$(dotd dag check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "shellrc.base"    || { printf 'FAIL: shellrc.base not in dag check output\n'; exit 1; }
printf '%s' "$OUT" | grep -q "shellrc.path"    || { printf 'FAIL: shellrc.path not in dag check output\n'; exit 1; }
printf '%s' "$OUT" | grep -q "shellrc.aliases" || { printf 'FAIL: shellrc.aliases not in dag check output\n'; exit 1; }

BASE_LINE=$(printf '%s' "$OUT" | grep -n "shellrc.base" | head -1 | cut -d: -f1)
PATH_LINE=$(printf '%s' "$OUT" | grep -n "shellrc.path" | head -1 | cut -d: -f1)
[ "$BASE_LINE" -lt "$PATH_LINE" ] \
  || { printf 'FAIL: shellrc.base (%s) should appear before shellrc.path (%s)\n' "$BASE_LINE" "$PATH_LINE"; exit 1; }

printf 'PASS: dag-check test\n'
EOF
chmod +x test/e2e/dag-check.sh
```

- [ ] **Step 14.4: Create `test/e2e/package-check.sh`**

The fixture `packages.yaml` defines `fake-installed` (binary=sh, always on PATH) and `not-installable` (no package manager, binary not on PATH).

```bash
cat > test/e2e/package-check.sh << 'EOF'
#!/bin/sh
set -e

OUT=$(dotd package check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "fake-installed" \
  || { printf 'FAIL: fake-installed not in package check output\n'; exit 1; }
printf '%s' "$OUT" | grep -q "installed" \
  || { printf 'FAIL: installed status not in output\n'; exit 1; }

# Verify package generate fails when a @require package has no installable manager.
cp -r /fixture /tmp/pkgdotfiles
printf '#!/bin/bash\n# @require(not-installable)\necho hi\n' \
  > /tmp/pkgdotfiles/shellrc/hard-fail.sh

if dotd package generate \
  --files /tmp/pkgdotfiles \
  --env-file /tmp/pkgdotfiles/env.yaml \
  --env os=linux 2>/dev/null; then
  printf 'FAIL: package generate should fail with uninstallable @require\n'
  exit 1
fi

printf 'PASS: package-check test\n'
EOF
chmod +x test/e2e/package-check.sh
```

- [ ] **Step 14.5: Commit shell e2e scripts**

```bash
git add test/e2e/adopt.sh test/e2e/bundle.sh test/e2e/dag-check.sh test/e2e/package-check.sh
git commit -m "test(e2e): add adopt, bundle, dag-check, package-check shell e2e tests"
```

---

## Task 15: Wire Batch 5 into Dockerfiles and runners, run and push

**Files:** Modify `test/e2e/Dockerfile`, `test/e2e/Dockerfile.local`, `test/run-e2e.sh`, `test/run-e2e-release.sh`

- [ ] **Step 15.1: Add COPY lines to both Dockerfiles**

After the Batch 4 COPY lines, add:

```
COPY adopt.sh /tests/adopt.sh
COPY bundle.sh /tests/bundle.sh
COPY dag-check.sh /tests/dag-check.sh
COPY package-check.sh /tests/package-check.sh
```

- [ ] **Step 15.2: Add `run_test` calls to both runners**

After the Batch 4 `run_test` lines, add:

```sh
run_test adopt.sh
run_test bundle.sh
run_test dag-check.sh
run_test package-check.sh
```

- [ ] **Step 15.3: Run the full e2e suite**

```bash
./test/run-e2e.sh
```

Expected: `PASS: adopt test`, `PASS: bundle test`, `PASS: dag-check test`, `PASS: package-check test` all appear; `All e2e tests passed.`

- [ ] **Step 15.4: Run all integration tests together as final check**

```bash
go test -tags integration -count=1 ./cmd/dotd/ -v 2>&1 | grep -E "^(--- PASS|--- FAIL|FAIL|ok)"
```

Expected: all lines are `--- PASS: ...` with `ok cmd/dotd/...` at the end.

- [ ] **Step 15.5: Commit, verify PR open, push**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local test/run-e2e.sh test/run-e2e-release.sh
git commit -m "test(e2e): wire adopt/bundle/dag-check/package-check into Dockerfiles and runners (Batch 5)"
gh pr view
git push
```

---

## Final Dockerfile reference state

After all 5 batches, both Dockerfiles should have these COPY lines for test scripts (in addition to `COPY procure/ /procure/`):

```
COPY apply.sh /tests/apply.sh
COPY context.sh /tests/context.sh
COPY dag-order.sh /tests/dag-order.sh
COPY dry-run.sh /tests/dry-run.sh
COPY idempotent.sh /tests/idempotent.sh
COPY check.sh /tests/check.sh
COPY list.sh /tests/list.sh
COPY bin.sh /tests/bin.sh
COPY symlinks-nested.sh /tests/symlinks-nested.sh
COPY disable.sh /tests/disable.sh
COPY packages.sh /tests/packages.sh
COPY conflict.sh /tests/conflict.sh
COPY unapply.sh /tests/unapply.sh
COPY unapply-cancel.sh /tests/unapply-cancel.sh
COPY compose.sh /tests/compose.sh
COPY macos-apply.sh /tests/macos-apply.sh
COPY setup.sh /tests/setup.sh
COPY teardown-confirm.sh /tests/teardown-confirm.sh
COPY teardown-cancel.sh /tests/teardown-cancel.sh
COPY init.sh /tests/init.sh
COPY config-cmds.sh /tests/config-cmds.sh
COPY env-cmds.sh /tests/env-cmds.sh
COPY adopt.sh /tests/adopt.sh
COPY bundle.sh /tests/bundle.sh
COPY dag-check.sh /tests/dag-check.sh
COPY package-check.sh /tests/package-check.sh
```
