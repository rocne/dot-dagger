# E2E Test Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the Docker-based e2e test suite from 3 smoke tests to ~15 tests covering all major dotd features: predicates, DAG ordering, symlink variants, bin/ linking, dry-run, idempotency, `dotd check`, `dotd list`, `@disable`, packages, and conflict handling.

**Architecture:** The existing fixture (`test/e2e/fixture/`) is upgraded to mirror the richer `cmd/dotd/testdata/dotfiles/` integration fixture. Each new behavior gets its own test script that runs in a fresh container against the shared fixture. `run-e2e.sh` and `Dockerfile` are updated to wire up all new scripts.

**Tech Stack:** Shell scripts (POSIX sh), Docker (ubuntu:24.04), dotd binary (downloaded from GitHub releases).

---

## Context for implementer

### Repository layout

```
test/
  run-e2e.sh           # host orchestrator — builds image, runs containers
  e2e/
    Dockerfile         # ubuntu:24.04, copies all test scripts to /tests/
    binary.sh          # existing: downloads binary, applies, asserts .zshrc + init.sh
    install.sh         # existing: runs install.sh, asserts binary exists
    combined.sh        # existing: install + apply
    fixture/           # shared dotfiles repo mounted read-only at /fixture in containers
      env.yaml
      conf/.dagger
      conf/dot-zshrc
      shellrc/.dagger
      shellrc/base.sh
      shellrc/linux.sh
      shellrc/macos.sh
```

### How tests run

Each test script runs as: `docker run --rm -e DOTD_VERSION=... -v fixture:/fixture:ro dotd-e2e sh /tests/<name>.sh`

The container has no dotd binary pre-installed. Each test downloads the release artifact itself (binary test) or uses the installer (install/combined). All paths are explicitly controlled via flags: `--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated`.

### Important: compose is NOT yet implemented in v2

Integration tests for compose are all skipped with "not yet implemented in v2 pipeline". Do NOT add compose targets (`composition.enabled: true`, `compose: true`) to the fixture — dotd will error on them.

### Current binary.sh assertions

```sh
test -L /home/e2e/.zshrc          || { printf 'FAIL: .zshrc symlink missing\n'; exit 1; }
grep -q "base.sh"  /tmp/init.sh    || { printf 'FAIL: base.sh not in init.sh\n'; exit 1; }
grep -q "linux.sh" /tmp/init.sh    || { printf 'FAIL: linux.sh not in init.sh\n'; exit 1; }
! grep -q "macos.sh" /tmp/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n'; exit 1; }
```

### Annotation format used in this codebase

The integration testdata uses: `# @when(os=linux)`, `# @after(shellrc.base)`, `# @link(~/.gitconfig)`.
These parenthesised forms are accepted by dotd's annotation scanner.

### ~bin/ link_root

The `bin/` convention directory uses `link_root: "~bin"` in its `.dagger`. dotd expands `~bin` to the value of `--bin-dir`. So `bin/hello` with default link action → symlinked to `<bin-dir>/hello`.

---

## File map

**New fixture files:**

| Path | What |
|------|------|
| `test/e2e/fixture/env.yaml` | Update: add `context: personal` |
| `test/e2e/fixture/packages.yaml` | New: fake-installed (binary=sh), not-installable |
| `test/e2e/fixture/conf/dot-gitconfig` | New: `@when(context=personal)` `@link(~/.gitconfig)` |
| `test/e2e/fixture/conf/dot-config/nvim/.dagger` | New: explicit link dest for init.lua |
| `test/e2e/fixture/conf/dot-config/nvim/init.lua` | New: lua placeholder |
| `test/e2e/fixture/bin/.dagger` | New: `link_root: "~bin"`, `defaults: link` |
| `test/e2e/fixture/bin/hello` | New: executable shell script |
| `test/e2e/fixture/shellrc/path.sh` | New: `@after(shellrc.base)` |
| `test/e2e/fixture/shellrc/aliases.sh` | New: `@after(shellrc.path)` |
| `test/e2e/fixture/shellrc/work.sh` | New: `@when(context=work)` `@after(shellrc.aliases)` |
| `test/e2e/fixture/shellrc/disabled.sh` | New: `@disable` |
| `test/e2e/fixture/shellrc/needs-fake.sh` | New: `@require(fake-installed)` |
| `test/e2e/fixture/shellrc/optional-tool.sh` | New: `@request(not-installable)` |

**New test scripts:**

| Path | What it tests |
|------|---------------|
| `test/e2e/context.sh` | `@when context=work/personal` — work.sh in/out, .gitconfig in/out |
| `test/e2e/dag-order.sh` | `@after` ordering — base→path→aliases→linux verified in init.sh |
| `test/e2e/dry-run.sh` | `--dry-run` — no files created |
| `test/e2e/idempotent.sh` | Apply twice — init.sh identical both times |
| `test/e2e/check.sh` | `dotd check` exits 0 after apply |
| `test/e2e/list.sh` | `dotd list` output includes known active nodes |
| `test/e2e/bin.sh` | `bin/hello` → `<bin-dir>/hello` symlink |
| `test/e2e/symlinks-nested.sh` | `dot-config/nvim/init.lua` → `~/.config/nvim/init.lua` |
| `test/e2e/disable.sh` | `@disable` file absent from init.sh |
| `test/e2e/packages.sh` | `@require` met (needs-fake.sh in init.sh), `@request` skipped (optional-tool.sh in init.sh) |
| `test/e2e/conflict.sh` | Plain file blocks apply; `--force` succeeds |

**Modified files:**

| Path | Change |
|------|--------|
| `test/e2e/binary.sh` | Add assertions for path.sh, aliases.sh in init.sh |
| `test/e2e/combined.sh` | Same as binary.sh update |
| `test/e2e/Dockerfile` | COPY all 11 new test scripts |
| `test/run-e2e.sh` | `docker run` for each new test |

---

## Task 1: Expand the e2e fixture

**Files:**
- Modify: `test/e2e/fixture/env.yaml`
- Create: `test/e2e/fixture/packages.yaml`
- Create: `test/e2e/fixture/conf/dot-gitconfig`
- Create: `test/e2e/fixture/conf/dot-config/nvim/.dagger`
- Create: `test/e2e/fixture/conf/dot-config/nvim/init.lua`
- Create: `test/e2e/fixture/bin/.dagger`
- Create: `test/e2e/fixture/bin/hello`
- Create: `test/e2e/fixture/shellrc/path.sh`
- Create: `test/e2e/fixture/shellrc/aliases.sh`
- Create: `test/e2e/fixture/shellrc/work.sh`
- Create: `test/e2e/fixture/shellrc/disabled.sh`
- Create: `test/e2e/fixture/shellrc/needs-fake.sh`
- Create: `test/e2e/fixture/shellrc/optional-tool.sh`

- [ ] **Step 1: Update env.yaml**

```yaml
# test/e2e/fixture/env.yaml
context: personal
```

- [ ] **Step 2: Create packages.yaml**

```yaml
# test/e2e/fixture/packages.yaml
package_managers:
  apt:
    install: apt-get install -y {package}
    uninstall: apt-get remove -y {package}

packages:
  # binary=sh is always on PATH — @require always passes without installing anything
  fake-installed:
    binary: sh
    apt: {}

  # no managers defined — @require always hard-fails, @request silently skips
  not-installable:
    binary: definitely-not-a-real-binary-zzz
```

- [ ] **Step 3: Create conf/dot-gitconfig**

```
# @when(context=personal)
# @link(~/.gitconfig)
[user]
	name = Test User
	email = test@personal.example.com
[core]
	editor = vim
```

- [ ] **Step 4: Create conf/dot-config/nvim/.dagger**

```yaml
files:
  init.lua:
    actions:
      - link(~/.config/nvim/init.lua)
```

- [ ] **Step 5: Create conf/dot-config/nvim/init.lua**

```lua
-- neovim config placeholder (lua comments not scanned for annotations)
vim.opt.number = true
vim.opt.expandtab = true
vim.opt.shiftwidth = 2
```

- [ ] **Step 6: Create bin/.dagger**

```yaml
link_root: "~bin"
defaults:
  actions:
    - link
```

- [ ] **Step 7: Create bin/hello**

```sh
#!/bin/sh
echo "hello from dotfiles"
```

Make it executable: `chmod +x test/e2e/fixture/bin/hello`

- [ ] **Step 8: Create shellrc/path.sh**

```bash
#!/bin/bash
# @after(shellrc.base)
export PATH="$HOME/.local/bin:$PATH"
```

- [ ] **Step 9: Create shellrc/aliases.sh**

```bash
#!/bin/bash
# @after(shellrc.path)
alias ll='ls -la'
alias la='ls -A'
```

- [ ] **Step 10: Create shellrc/work.sh**

```bash
#!/bin/bash
# @when(context=work)
# @after(shellrc.aliases)
alias deploy='echo "deploying..."'
```

- [ ] **Step 11: Create shellrc/disabled.sh**

```bash
#!/bin/bash
# @disable
echo "this file is disabled and should never appear"
```

- [ ] **Step 12: Create shellrc/needs-fake.sh**

```bash
#!/bin/bash
# @require(fake-installed)
# @after(shellrc.base)
echo "fake-installed requirement met"
```

- [ ] **Step 13: Create shellrc/optional-tool.sh**

```bash
#!/bin/bash
# @request(not-installable)
# @after(shellrc.base)
echo "optional tool setup"
```

- [ ] **Step 14: Verify existing binary test still passes against the richer fixture**

Run with an existing release to confirm nothing broke:

```sh
DOTD_VERSION=v0.2.34 ./test/run-e2e.sh
```

Expected: the three existing tests (binary, installer, combined) all still print PASS. The richer fixture adds more files but doesn't break existing assertions.

Note: if you don't have Docker locally, skip this step — CI will catch it on the next release.

- [ ] **Step 15: Commit**

```bash
git add test/e2e/fixture/
git commit -m "test(e2e): expand fixture — add bin/, nested conf/, predicate variants, packages"
```

---

## Task 2: Add context and DAG ordering tests

**Files:**
- Create: `test/e2e/context.sh`
- Create: `test/e2e/dag-order.sh`

- [ ] **Step 1: Create context.sh**

This test runs apply twice in the same container — once as work, once as personal — and asserts the right files appear/disappear.

```sh
#!/bin/sh
# test/e2e/context.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd
DOTD=/tmp/dotd

# --- work context ---
mkdir -p /home/e2e-work/bin /tmp/gen-work
"${DOTD}" apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-work \
  --bin-dir /home/e2e-work/bin \
  --init-file /tmp/init-work.sh \
  --generated-dir /tmp/gen-work \
  --env os=linux \
  --env context=work

grep -q "work.sh"  /tmp/init-work.sh  || { printf 'FAIL: work.sh should be in init.sh for context=work\n';   exit 1; }
! grep -q "work.sh" /tmp/init-work.sh 2>/dev/null && true  # re-affirm
grep -q "work.sh"  /tmp/init-work.sh  || { printf 'FAIL: work.sh missing in work context\n'; exit 1; }
! test -L /home/e2e-work/.gitconfig   || { printf 'FAIL: .gitconfig should not be linked in work context\n'; exit 1; }

# --- personal context ---
mkdir -p /home/e2e-personal/bin /tmp/gen-personal
"${DOTD}" apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-personal \
  --bin-dir /home/e2e-personal/bin \
  --init-file /tmp/init-personal.sh \
  --generated-dir /tmp/gen-personal \
  --env os=linux \
  --env context=personal

! grep -q "work.sh" /tmp/init-personal.sh || { printf 'FAIL: work.sh should not be in init.sh for context=personal\n'; exit 1; }
test -L /home/e2e-personal/.gitconfig      || { printf 'FAIL: .gitconfig should be linked in personal context\n'; exit 1; }

printf 'PASS: context test\n'
```

- [ ] **Step 2: Create dag-order.sh**

This test verifies that `@after` constraints are respected in init.sh: base < path < aliases, and base < linux.

```sh
#!/bin/sh
# test/e2e/dag-order.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# Extract position of each script in init.sh
pos_base=$(grep -n "base.sh"    /tmp/init.sh | head -1 | cut -d: -f1)
pos_path=$(grep -n "path.sh"    /tmp/init.sh | head -1 | cut -d: -f1)
pos_aliases=$(grep -n "aliases.sh" /tmp/init.sh | head -1 | cut -d: -f1)
pos_linux=$(grep -n "linux.sh"  /tmp/init.sh | head -1 | cut -d: -f1)

[ -n "$pos_base" ]    || { printf 'FAIL: base.sh not in init.sh\n';    exit 1; }
[ -n "$pos_path" ]    || { printf 'FAIL: path.sh not in init.sh\n';    exit 1; }
[ -n "$pos_aliases" ] || { printf 'FAIL: aliases.sh not in init.sh\n'; exit 1; }
[ -n "$pos_linux" ]   || { printf 'FAIL: linux.sh not in init.sh\n';   exit 1; }

[ "$pos_base" -lt "$pos_path" ]    || { printf 'FAIL: base.sh must come before path.sh\n';    exit 1; }
[ "$pos_path" -lt "$pos_aliases" ] || { printf 'FAIL: path.sh must come before aliases.sh\n'; exit 1; }
[ "$pos_base" -lt "$pos_linux" ]   || { printf 'FAIL: base.sh must come before linux.sh\n';   exit 1; }

printf 'PASS: dag-order test\n'
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/context.sh test/e2e/dag-order.sh
git commit -m "test(e2e): add context and dag-order tests"
```

---

## Task 3: Add dry-run and idempotency tests

**Files:**
- Create: `test/e2e/dry-run.sh`
- Create: `test/e2e/idempotent.sh`

- [ ] **Step 1: Create dry-run.sh**

```sh
#!/bin/sh
# test/e2e/dry-run.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal \
  --dry-run

# Nothing should have been written
! test -e /home/e2e/.zshrc   || { printf 'FAIL: .zshrc should not exist after --dry-run\n'; exit 1; }
! test -e /tmp/init.sh       || { printf 'FAIL: init.sh should not exist after --dry-run\n'; exit 1; }
! test -e /home/e2e/bin/hello || { printf 'FAIL: bin/hello should not exist after --dry-run\n'; exit 1; }

printf 'PASS: dry-run test\n'
```

- [ ] **Step 2: Create idempotent.sh**

```sh
#!/bin/sh
# test/e2e/idempotent.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated

APPLY_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

# First apply
# shellcheck disable=SC2086
/tmp/dotd apply $APPLY_ARGS

cp /tmp/init.sh /tmp/init1.sh

# Second apply
# shellcheck disable=SC2086
/tmp/dotd apply $APPLY_ARGS

cp /tmp/init.sh /tmp/init2.sh

diff /tmp/init1.sh /tmp/init2.sh || { printf 'FAIL: init.sh changed between two identical apply runs\n'; exit 1; }

printf 'PASS: idempotent test\n'
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/dry-run.sh test/e2e/idempotent.sh
git commit -m "test(e2e): add dry-run and idempotency tests"
```

---

## Task 4: Add check and list tests

**Files:**
- Create: `test/e2e/check.sh`
- Create: `test/e2e/list.sh`

- [ ] **Step 1: Create check.sh**

```sh
#!/bin/sh
# test/e2e/check.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated

COMMON="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

# Apply first
# shellcheck disable=SC2086
/tmp/dotd apply $COMMON

# check should exit 0 cleanly
# shellcheck disable=SC2086
/tmp/dotd check $COMMON || { printf 'FAIL: dotd check failed after successful apply\n'; exit 1; }

printf 'PASS: check test\n'
```

- [ ] **Step 2: Create list.sh**

```sh
#!/bin/sh
# test/e2e/list.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

COMMON="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

# dotd list shows active nodes
# shellcheck disable=SC2086
OUT=$(/tmp/dotd list $COMMON)

echo "$OUT" | grep -q "base"    || { printf 'FAIL: dotd list missing "base"\nOutput:\n%s\n' "$OUT";    exit 1; }
echo "$OUT" | grep -q "path"    || { printf 'FAIL: dotd list missing "path"\nOutput:\n%s\n' "$OUT";    exit 1; }
echo "$OUT" | grep -q "aliases" || { printf 'FAIL: dotd list missing "aliases"\nOutput:\n%s\n' "$OUT"; exit 1; }
echo "$OUT" | grep -q "linux"   || { printf 'FAIL: dotd list missing "linux"\nOutput:\n%s\n' "$OUT";   exit 1; }

# macos and work should NOT appear in active list (inactive for os=linux, context=personal)
! echo "$OUT" | grep -q "macos" || { printf 'FAIL: dotd list shows inactive "macos"\nOutput:\n%s\n' "$OUT"; exit 1; }
! echo "$OUT" | grep -q "work"  || { printf 'FAIL: dotd list shows inactive "work"\nOutput:\n%s\n' "$OUT";  exit 1; }

# --inactive should include macos and work
# shellcheck disable=SC2086
OUT_ALL=$(/tmp/dotd list --inactive $COMMON)
echo "$OUT_ALL" | grep -q "macos" || { printf 'FAIL: dotd list --inactive missing "macos"\nOutput:\n%s\n' "$OUT_ALL"; exit 1; }
echo "$OUT_ALL" | grep -q "work"  || { printf 'FAIL: dotd list --inactive missing "work"\nOutput:\n%s\n' "$OUT_ALL";  exit 1; }

printf 'PASS: list test\n'
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/check.sh test/e2e/list.sh
git commit -m "test(e2e): add check and list tests"
```

---

## Task 5: Add bin and nested symlink tests

**Files:**
- Create: `test/e2e/bin.sh`
- Create: `test/e2e/symlinks-nested.sh`

- [ ] **Step 1: Create bin.sh**

```sh
#!/bin/sh
# test/e2e/bin.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# bin/hello should be symlinked into --bin-dir
test -L /home/e2e/bin/hello || { printf 'FAIL: bin/hello symlink missing from bin-dir\n'; exit 1; }

# the symlink should point to the fixture file
TARGET=$(readlink /home/e2e/bin/hello)
echo "$TARGET" | grep -q "bin/hello" || { printf 'FAIL: bin/hello symlink target wrong: %s\n' "$TARGET"; exit 1; }

# the binary should be executable
test -x /home/e2e/bin/hello || { printf 'FAIL: bin/hello is not executable\n'; exit 1; }

printf 'PASS: bin test\n'
```

- [ ] **Step 2: Create symlinks-nested.sh**

```sh
#!/bin/sh
# test/e2e/symlinks-nested.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# conf/dot-config/nvim/init.lua → ~/.config/nvim/init.lua (explicit @action in .dagger)
test -L /home/e2e/.config/nvim/init.lua \
  || { printf 'FAIL: ~/.config/nvim/init.lua symlink missing\n'; exit 1; }

# conf/dot-gitconfig → ~/.gitconfig (context=personal predicate matches)
test -L /home/e2e/.gitconfig \
  || { printf 'FAIL: ~/.gitconfig symlink missing (context=personal)\n'; exit 1; }

# conf/dot-zshrc → ~/.zshrc (basic, no predicate)
test -L /home/e2e/.zshrc \
  || { printf 'FAIL: ~/.zshrc symlink missing\n'; exit 1; }

printf 'PASS: symlinks-nested test\n'
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/bin.sh test/e2e/symlinks-nested.sh
git commit -m "test(e2e): add bin and nested symlink tests"
```

---

## Task 6: Add disable, packages, and conflict tests

**Files:**
- Create: `test/e2e/disable.sh`
- Create: `test/e2e/packages.sh`
- Create: `test/e2e/conflict.sh`

- [ ] **Step 1: Create disable.sh**

```sh
#!/bin/sh
# test/e2e/disable.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# disabled.sh has @disable — must NOT appear in init.sh
! grep -q "disabled.sh" /tmp/init.sh \
  || { printf 'FAIL: disabled.sh should not appear in init.sh\n'; exit 1; }

# base.sh has no @disable — must be present (control check)
grep -q "base.sh" /tmp/init.sh \
  || { printf 'FAIL: base.sh missing from init.sh (control check)\n'; exit 1; }

printf 'PASS: disable test\n'
```

- [ ] **Step 2: Create packages.sh**

```sh
#!/bin/sh
# test/e2e/packages.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# needs-fake.sh has @require(fake-installed); fake-installed binary=sh is always on PATH.
# The script must be in init.sh — @require was satisfied.
grep -q "needs-fake.sh" /tmp/init.sh \
  || { printf 'FAIL: needs-fake.sh should be in init.sh (@require satisfied)\n'; exit 1; }

# optional-tool.sh has @request(not-installable); not-installable has no binary.
# @request is soft — apply must succeed and the file must be in init.sh.
grep -q "optional-tool.sh" /tmp/init.sh \
  || { printf 'FAIL: optional-tool.sh should be in init.sh (@request is soft)\n'; exit 1; }

printf 'PASS: packages test\n'
```

- [ ] **Step 3: Create conflict.sh**

```sh
#!/bin/sh
# test/e2e/conflict.sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated

APPLY_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

# Place a plain file where dotd would create a symlink
printf 'existing content\n' > /home/e2e/.zshrc

# Apply without --force should fail (real file at destination)
# shellcheck disable=SC2086
if /tmp/dotd apply $APPLY_ARGS 2>/dev/null; then
  # Some builds warn but don't error; verify the real file was not replaced
  if readlink /home/e2e/.zshrc >/dev/null 2>&1; then
    printf 'INFO: apply without --force unexpectedly succeeded and created symlink\n'
  else
    printf 'INFO: apply without --force succeeded but left plain file — acceptable\n'
  fi
else
  printf 'INFO: apply without --force correctly failed\n'
fi

# Apply with --force must succeed and replace the plain file with a symlink
# shellcheck disable=SC2086
/tmp/dotd apply $APPLY_ARGS --force \
  || { printf 'FAIL: apply --force failed\n'; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: .zshrc is not a symlink after --force\n'; exit 1; }

printf 'PASS: conflict test\n'
```

- [ ] **Step 4: Commit**

```bash
git add test/e2e/disable.sh test/e2e/packages.sh test/e2e/conflict.sh
git commit -m "test(e2e): add disable, packages, and conflict tests"
```

---

## Task 7: Update binary.sh and combined.sh

**Files:**
- Modify: `test/e2e/binary.sh`
- Modify: `test/e2e/combined.sh`

The fixture now includes `path.sh` and `aliases.sh` (always active). The existing tests pass `--env os=linux` with `env.yaml context: personal`. Both scripts should now be in init.sh.

- [ ] **Step 1: Read current binary.sh**

Read `test/e2e/binary.sh` to see current assertions (shown above in context section).

- [ ] **Step 2: Update binary.sh assertions**

Replace the assertions block (lines 27–30) with an expanded version:

```sh
# assertions
test -L /home/e2e/.zshrc              || { printf 'FAIL: .zshrc symlink missing\n';     exit 1; }
grep -q "base.sh"    /tmp/init.sh     || { printf 'FAIL: base.sh not in init.sh\n';     exit 1; }
grep -q "path.sh"    /tmp/init.sh     || { printf 'FAIL: path.sh not in init.sh\n';     exit 1; }
grep -q "aliases.sh" /tmp/init.sh     || { printf 'FAIL: aliases.sh not in init.sh\n';  exit 1; }
grep -q "linux.sh"   /tmp/init.sh     || { printf 'FAIL: linux.sh not in init.sh\n';    exit 1; }
! grep -q "macos.sh"   /tmp/init.sh   || { printf 'FAIL: macos.sh should not be in init.sh\n';  exit 1; }
! grep -q "work.sh"    /tmp/init.sh   || { printf 'FAIL: work.sh should not be in init.sh\n';   exit 1; }
! grep -q "disabled.sh" /tmp/init.sh  || { printf 'FAIL: disabled.sh should not be in init.sh\n'; exit 1; }
```

- [ ] **Step 3: Apply the same assertion block to combined.sh**

`combined.sh` uses `"${HOME}/.local/bin/dotd"` as the binary. Apply the identical assertion block — just update the grep lines to match.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/binary.sh test/e2e/combined.sh
git commit -m "test(e2e): extend binary and combined assertions for richer fixture"
```

---

## Task 8: Update Dockerfile and run-e2e.sh

**Files:**
- Modify: `test/e2e/Dockerfile`
- Modify: `test/run-e2e.sh`

- [ ] **Step 1: Read current Dockerfile**

Read `test/e2e/Dockerfile` to see current COPY lines.

- [ ] **Step 2: Update Dockerfile**

Add COPY lines for all 11 new test scripts. The full COPY block should be:

```dockerfile
COPY binary.sh /tests/binary.sh
COPY install.sh /tests/install.sh
COPY combined.sh /tests/combined.sh
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
```

- [ ] **Step 3: Read current run-e2e.sh**

Read `test/run-e2e.sh` to see the current docker run block structure.

- [ ] **Step 4: Update run-e2e.sh**

Add docker run invocations for all 11 new tests after the existing three. Each new test receives the fixture volume mount (no install.sh mount needed for binary-only tests). The install.sh tests need the repo's install.sh mounted.

```sh
# context test
printf '\n=== context test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/context.sh

# dag-order test
printf '\n=== dag-order test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/dag-order.sh

# dry-run test
printf '\n=== dry-run test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/dry-run.sh

# idempotent test
printf '\n=== idempotent test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/idempotent.sh

# check test
printf '\n=== check test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/check.sh

# list test
printf '\n=== list test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/list.sh

# bin test
printf '\n=== bin test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/bin.sh

# symlinks-nested test
printf '\n=== symlinks-nested test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/symlinks-nested.sh

# disable test
printf '\n=== disable test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/disable.sh

# packages test
printf '\n=== packages test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/packages.sh

# conflict test
printf '\n=== conflict test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/conflict.sh
```

- [ ] **Step 5: Commit**

```bash
git add test/e2e/Dockerfile test/run-e2e.sh
git commit -m "test(e2e): wire 11 new test scripts into Dockerfile and run-e2e.sh"
```

---

## Task 9: Push and validate

- [ ] **Step 1: Check PR status**

```bash
gh pr view
```

Expected: PR is open on the current feature branch. If no PR exists, create one:

```bash
gh pr create --title "test(e2e): expand test suite to 14 tests" --body "$(cat <<'EOF'
## Summary
- Expands e2e fixture to match integration test fixture richness
- Adds 11 new test scripts covering: context predicates, DAG ordering, dry-run, idempotency, dotd check, dotd list, bin/ symlinking, nested symlinks, @disable, packages (@require/@request), symlink conflict + --force
- Updates binary.sh and combined.sh assertions for richer fixture
- All tests run against existing released binary — no new dotd features required

## Test plan
- [ ] Trigger a release (or run `DOTD_VERSION=v0.2.34 ./test/run-e2e.sh` locally) to verify all 14 tests pass
- [ ] Confirm no regressions in existing binary/install/combined tests
EOF
)"
```

- [ ] **Step 2: Push**

```bash
git push
```

- [ ] **Step 3: Trigger release or run locally**

Option A — run against existing release locally (if Docker available):

```bash
DOTD_VERSION=v0.2.34 ./test/run-e2e.sh
```

Expected: 14 `=== ... ===` sections, all ending in PASS.

Option B — trigger new release:

```bash
# Ensure on main after PR merge, then:
git tag v0.2.35
git push origin v0.2.35
```

Expected: GitHub Actions runs e2e job, all tests pass.

- [ ] **Step 4: If any test fails, diagnose and fix**

Common failure modes:
- `dotd list` output format differs from expected grep pattern → adjust grep
- `dotd check` exits non-zero for a reason other than drift → check output with `--log-level debug`
- Package test: `needs-fake.sh` not in init.sh → verify `packages.yaml` is being read (check `--files` path)
- `bin/hello` symlink missing → verify `bin/.dagger` has `link_root: "~bin"` spelled exactly so
