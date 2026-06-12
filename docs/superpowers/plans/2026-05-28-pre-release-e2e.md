# Pre-Release E2E Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor e2e tests into procurer + exerciser roles so the same exerciser scripts run against both a locally-built binary (pre-release, on every PR) and a GitHub-published binary (post-release).

**Architecture:** Each exerciser test runs in a fresh Docker container that first sources a procurer script to put `dotd` on PATH, then runs the exerciser. The local procurer copies a pre-built binary baked into the image; the release procurer runs `install.sh` inside the container like a real user would. CI gains an `e2e` job that builds the binary and runs the pre-release path on every PR.

**Tech Stack:** sh, Docker, Go cross-compilation (`GOOS=linux GOARCH=amd64`), GitHub Actions

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `test/e2e/procure/release.sh` | Create | Runs `install.sh --version $TAG`, asserts binary present, exports PATH |
| `test/e2e/procure/local.sh` | Create | Copies `/staged/dotd` to `~/.local/bin/dotd`, exports PATH |
| `test/e2e/apply.sh` | Create (from binary.sh) | Core apply exerciser, no download step |
| `test/e2e/context.sh` | Modify | Drop download block, `dotd` not `/tmp/dotd` |
| `test/e2e/dag-order.sh` | Modify | Same |
| `test/e2e/dry-run.sh` | Modify | Same |
| `test/e2e/idempotent.sh` | Modify | Same |
| `test/e2e/check.sh` | Modify | Same |
| `test/e2e/list.sh` | Modify | Same |
| `test/e2e/bin.sh` | Modify | Same |
| `test/e2e/symlinks-nested.sh` | Modify | Same |
| `test/e2e/disable.sh` | Modify | Same |
| `test/e2e/packages.sh` | Modify | Same |
| `test/e2e/conflict.sh` | Modify | Same |
| `test/e2e/Dockerfile` | Modify | Add `COPY procure/`, swap `binary.sh`→`apply.sh`, remove `install.sh`/`combined.sh` |
| `test/e2e/Dockerfile.local` | Create | Standalone image with staged binary baked in |
| `test/e2e/binary.sh` | Delete | Superseded by `apply.sh` |
| `test/e2e/install.sh` | Delete | Superseded by `procure/release.sh` |
| `test/e2e/combined.sh` | Delete | Redundant with procure/release.sh + apply.sh |
| `test/run-e2e.sh` | Modify | Rewrite to use procurer pattern |
| `test/run-e2e-local.sh` | Create | Pre-release runner: cross-compile + local Docker image |
| `.github/workflows/ci.yml` | Modify | Add `e2e` job after `test` |

---

## Task 1: Create procurer scripts

**Files:**
- Create: `test/e2e/procure/release.sh`
- Create: `test/e2e/procure/local.sh`

- [ ] **Step 1: Create `test/e2e/procure/release.sh`**

```sh
#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

sh /repo/install.sh --version "${TAG}"

test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed at ~/.local/bin/dotd\n'; exit 1; }
"${HOME}/.local/bin/dotd" --version || { printf 'FAIL: dotd --version exited non-zero\n'; exit 1; }

export PATH="$HOME/.local/bin:$PATH"
```

- [ ] **Step 2: Create `test/e2e/procure/local.sh`**

```sh
#!/bin/sh
set -e

mkdir -p "${HOME}/.local/bin"
cp /staged/dotd "${HOME}/.local/bin/dotd"
chmod +x "${HOME}/.local/bin/dotd"

export PATH="$HOME/.local/bin:$PATH"
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/procure/
git commit -m "feat(e2e): add procurer scripts (release and local)"
```

---

## Task 2: Create apply.sh

`binary.sh` currently downloads the binary AND exercises it. The new `apply.sh` is the exerciser only — no download step. `binary.sh` stays until Task 7 to avoid breaking anything mid-refactor.

**Files:**
- Create: `test/e2e/apply.sh`

- [ ] **Step 1: Create `test/e2e/apply.sh`**

```sh
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
  --env os=linux

test -L /home/e2e/.zshrc              || { printf 'FAIL: .zshrc symlink missing\n';      exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: .zshrc symlink target wrong: %s\n' "$TARGET"; exit 1; }
grep -q "base.sh"    /tmp/init.sh     || { printf 'FAIL: base.sh not in init.sh\n';      exit 1; }
grep -q "path.sh"    /tmp/init.sh     || { printf 'FAIL: path.sh not in init.sh\n';      exit 1; }
grep -q "aliases.sh" /tmp/init.sh     || { printf 'FAIL: aliases.sh not in init.sh\n';   exit 1; }
grep -q "linux.sh"   /tmp/init.sh     || { printf 'FAIL: linux.sh not in init.sh\n';     exit 1; }
! grep -q "macos.sh"    /tmp/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n';    exit 1; }
! grep -q "work.sh"     /tmp/init.sh  || { printf 'FAIL: work.sh should not be in init.sh\n';     exit 1; }
! grep -q "disabled.sh" /tmp/init.sh  || { printf 'FAIL: disabled.sh should not be in init.sh\n'; exit 1; }

printf 'PASS: apply test\n'
```

- [ ] **Step 2: Commit**

```bash
git add test/e2e/apply.sh
git commit -m "feat(e2e): add apply.sh exerciser (no download step)"
```

---

## Task 3: Refactor exerciser scripts

Drop the download block from each script and replace `/tmp/dotd` with `dotd`. The download block is always the first 6 lines after `set -e` (TAG, ASSET, BASE_URL, curl, tar, chmod).

**Files:**
- Modify: `test/e2e/context.sh`
- Modify: `test/e2e/dag-order.sh`
- Modify: `test/e2e/dry-run.sh`
- Modify: `test/e2e/idempotent.sh`
- Modify: `test/e2e/check.sh`
- Modify: `test/e2e/list.sh`
- Modify: `test/e2e/bin.sh`
- Modify: `test/e2e/symlinks-nested.sh`
- Modify: `test/e2e/disable.sh`
- Modify: `test/e2e/packages.sh`
- Modify: `test/e2e/conflict.sh`

- [ ] **Step 1: Replace `test/e2e/context.sh`**

```sh
#!/bin/sh
set -e

# --- work context ---
mkdir -p /home/e2e-work/bin /tmp/gen-work
dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-work \
  --bin-dir /home/e2e-work/bin \
  --init-file /tmp/init-work.sh \
  --generated-dir /tmp/gen-work \
  --env os=linux \
  --env context=work

grep -q "work.sh" /tmp/init-work.sh \
  || { printf 'FAIL: work.sh should be in init.sh for context=work\n'; exit 1; }
! test -L /home/e2e-work/.gitconfig \
  || { printf 'FAIL: .gitconfig should not be linked in work context\n'; exit 1; }

# --- personal context ---
mkdir -p /home/e2e-personal/bin /tmp/gen-personal
dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-personal \
  --bin-dir /home/e2e-personal/bin \
  --init-file /tmp/init-personal.sh \
  --generated-dir /tmp/gen-personal \
  --env os=linux \
  --env context=personal

! grep -q "work.sh" /tmp/init-personal.sh \
  || { printf 'FAIL: work.sh should not be in init.sh for context=personal\n'; exit 1; }
test -L /home/e2e-personal/.gitconfig \
  || { printf 'FAIL: .gitconfig should be linked in personal context\n'; exit 1; }
TARGET=$(readlink /home/e2e-personal/.gitconfig)
[ "$TARGET" = "/fixture/conf/dot-gitconfig" ] \
  || { printf 'FAIL: .gitconfig target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: context test\n'
```

- [ ] **Step 2: Replace `test/e2e/dag-order.sh`**

```sh
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

- [ ] **Step 3: Replace `test/e2e/dry-run.sh`**

```sh
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
  --env context=personal \
  --dry-run

! test -e /home/e2e/.zshrc    || { printf 'FAIL: .zshrc should not exist after --dry-run\n'; exit 1; }
! test -e /tmp/init.sh        || { printf 'FAIL: init.sh should not exist after --dry-run\n'; exit 1; }
! test -e /home/e2e/bin/hello || { printf 'FAIL: bin/hello should not exist after --dry-run\n'; exit 1; }

printf 'PASS: dry-run test\n'
```

- [ ] **Step 4: Replace `test/e2e/idempotent.sh`**

```sh
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

cp /tmp/init.sh /tmp/init1.sh

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

diff /tmp/init1.sh /tmp/init.sh \
  || { printf 'FAIL: init.sh changed between two identical apply runs\n'; exit 1; }

printf 'PASS: idempotent test\n'
```

- [ ] **Step 5: Replace `test/e2e/check.sh`**

```sh
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

COMMON_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

dotd apply $COMMON_ARGS

dotd check $COMMON_ARGS \
  || { printf 'FAIL: dotd check failed after successful apply\n'; exit 1; }

printf 'PASS: check test\n'
```

- [ ] **Step 6: Replace `test/e2e/list.sh`**

```sh
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

COMMON_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

OUT=$(dotd list $COMMON_ARGS)
echo "$OUT" | grep -q "base"    || { printf 'FAIL: dotd list missing "base"\nOutput:\n%s\n'    "$OUT"; exit 1; }
echo "$OUT" | grep -q "path"    || { printf 'FAIL: dotd list missing "path"\nOutput:\n%s\n'    "$OUT"; exit 1; }
echo "$OUT" | grep -q "aliases" || { printf 'FAIL: dotd list missing "aliases"\nOutput:\n%s\n' "$OUT"; exit 1; }
echo "$OUT" | grep -q "linux"   || { printf 'FAIL: dotd list missing "linux"\nOutput:\n%s\n'   "$OUT"; exit 1; }

! echo "$OUT" | grep -q "macos" \
  || { printf 'FAIL: dotd list shows inactive "macos"\nOutput:\n%s\n' "$OUT"; exit 1; }
! echo "$OUT" | grep -q "work" \
  || { printf 'FAIL: dotd list shows inactive "work"\nOutput:\n%s\n' "$OUT"; exit 1; }

OUT_ALL=$(dotd list --inactive $COMMON_ARGS)
echo "$OUT_ALL" | grep -q "macos" \
  || { printf 'FAIL: dotd list --inactive missing "macos"\nOutput:\n%s\n' "$OUT_ALL"; exit 1; }
echo "$OUT_ALL" | grep -q "work" \
  || { printf 'FAIL: dotd list --inactive missing "work"\nOutput:\n%s\n' "$OUT_ALL";  exit 1; }

printf 'PASS: list test\n'
```

- [ ] **Step 7: Replace `test/e2e/bin.sh`**

```sh
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

test -L /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello symlink missing from bin-dir\n'; exit 1; }

TARGET=$(readlink /home/e2e/bin/hello)
[ "$TARGET" = "/fixture/bin/hello" ] \
  || { printf 'FAIL: bin/hello symlink target wrong: %s\n' "$TARGET"; exit 1; }

test -x /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello is not executable via symlink\n'; exit 1; }

printf 'PASS: bin test\n'
```

- [ ] **Step 8: Replace `test/e2e/symlinks-nested.sh`**

```sh
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

test -L /home/e2e/.config/nvim/init.lua \
  || { printf 'FAIL: ~/.config/nvim/init.lua symlink missing\n'; exit 1; }
TARGET=$(readlink /home/e2e/.config/nvim/init.lua)
[ "$TARGET" = "/fixture/conf/dot-config/nvim/init.lua" ] \
  || { printf 'FAIL: ~/.config/nvim/init.lua target wrong: %s\n' "$TARGET"; exit 1; }

test -L /home/e2e/.gitconfig \
  || { printf 'FAIL: ~/.gitconfig symlink missing (context=personal)\n'; exit 1; }
TARGET=$(readlink /home/e2e/.gitconfig)
[ "$TARGET" = "/fixture/conf/dot-gitconfig" ] \
  || { printf 'FAIL: ~/.gitconfig target wrong: %s\n' "$TARGET"; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: ~/.zshrc symlink missing\n'; exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: ~/.zshrc target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: symlinks-nested test\n'
```

- [ ] **Step 9: Replace `test/e2e/disable.sh`**

```sh
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

! grep -q "disabled.sh" /tmp/init.sh \
  || { printf 'FAIL: disabled.sh should not appear in init.sh\n'; exit 1; }

grep -q "base.sh" /tmp/init.sh \
  || { printf 'FAIL: base.sh missing from init.sh (control check)\n'; exit 1; }

printf 'PASS: disable test\n'
```

- [ ] **Step 10: Replace `test/e2e/packages.sh`**

```sh
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

grep -q "needs-fake.sh" /tmp/init.sh \
  || { printf 'FAIL: needs-fake.sh should be in init.sh (@require satisfied)\n'; exit 1; }

grep -q "optional-tool.sh" /tmp/init.sh \
  || { printf 'FAIL: optional-tool.sh should be in init.sh (@request is soft)\n'; exit 1; }

printf 'PASS: packages test\n'
```

- [ ] **Step 11: Replace `test/e2e/conflict.sh`**

```sh
#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

printf 'existing content\n' > /home/e2e/.zshrc

APPLY_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

dotd apply $APPLY_ARGS 2>/dev/null \
  && { printf 'FAIL: apply without --force should have failed on plain-file conflict\n'; exit 1; } \
  || true
test -L /home/e2e/.zshrc \
  && { printf 'FAIL: apply without --force must not replace plain file with symlink\n'; exit 1; } \
  || true

rm -f /home/e2e/.zshrc
printf 'existing content\n' > /home/e2e/.zshrc

dotd apply $APPLY_ARGS --force \
  || { printf 'FAIL: apply --force failed\n'; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: .zshrc is not a symlink after apply --force\n'; exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: .zshrc symlink target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: conflict test\n'
```

- [ ] **Step 12: Commit**

```bash
git add test/e2e/context.sh test/e2e/dag-order.sh test/e2e/dry-run.sh \
        test/e2e/idempotent.sh test/e2e/check.sh test/e2e/list.sh \
        test/e2e/bin.sh test/e2e/symlinks-nested.sh test/e2e/disable.sh \
        test/e2e/packages.sh test/e2e/conflict.sh
git commit -m "refactor(e2e): drop download blocks from exerciser scripts"
```

---

## Task 4: Update Dockerfiles

The existing `Dockerfile` currently COPYs `binary.sh`, `install.sh`, and `combined.sh` — all of which are changing. It also needs `COPY procure/ /procure/`. Note: `Dockerfile.local` must be **standalone** (not `FROM dotd-e2e`) since `dotd-e2e` is not pre-built in CI when `run-e2e-local.sh` runs.

**Files:**
- Modify: `test/e2e/Dockerfile`
- Create: `test/e2e/Dockerfile.local`

- [ ] **Step 1: Replace `test/e2e/Dockerfile`**

```dockerfile
FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
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
COPY procure/ /procure/
```

- [ ] **Step 2: Create `test/e2e/Dockerfile.local`**

```dockerfile
FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
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
COPY procure/ /procure/
COPY dotd /staged/dotd
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/Dockerfile test/e2e/Dockerfile.local
git commit -m "feat(e2e): update Dockerfiles for procurer/exerciser split"
```

---

## Task 5: Rewrite run-e2e.sh

**Files:**
- Modify: `test/run-e2e.sh`

- [ ] **Step 1: Replace `test/run-e2e.sh`**

```sh
#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [ -z "${DOTD_VERSION}" ]; then
  DOTD_VERSION="$(curl -fsSL "https://api.github.com/repos/rocne/dot-dagger/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

printf 'e2e: testing release %s\n' "${DOTD_VERSION}"

docker build -t dotd-e2e "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -e DOTD_VERSION="${DOTD_VERSION}" \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
    dotd-e2e \
    sh -c ". /procure/release.sh && sh /tests/${EXERCISER}"
}

run_test apply.sh
run_test context.sh
run_test dag-order.sh
run_test dry-run.sh
run_test idempotent.sh
run_test check.sh
run_test list.sh
run_test bin.sh
run_test symlinks-nested.sh
run_test disable.sh
run_test packages.sh
run_test conflict.sh

printf '\nAll e2e tests passed.\n'
```

- [ ] **Step 2: Commit**

```bash
git add test/run-e2e.sh
git commit -m "refactor(e2e): rewrite run-e2e.sh for procurer/exerciser pattern"
```

---

## Task 6: Create run-e2e-local.sh

**Files:**
- Create: `test/run-e2e-local.sh`

- [ ] **Step 1: Create `test/run-e2e-local.sh`**

```sh
#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

trap 'rm -f "${SCRIPT_DIR}/e2e/dotd"' EXIT

printf 'e2e-local: building dotd for linux/amd64\n'
GOOS=linux GOARCH=amd64 go build -o "${SCRIPT_DIR}/e2e/dotd" "${REPO_ROOT}/cmd/dotd"

printf 'e2e-local: building docker image\n'
docker build -t dotd-e2e-local -f "${SCRIPT_DIR}/e2e/Dockerfile.local" "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    dotd-e2e-local \
    sh -c ". /procure/local.sh && sh /tests/${EXERCISER}"
}

run_test apply.sh
run_test context.sh
run_test dag-order.sh
run_test dry-run.sh
run_test idempotent.sh
run_test check.sh
run_test list.sh
run_test bin.sh
run_test symlinks-nested.sh
run_test disable.sh
run_test packages.sh
run_test conflict.sh

printf '\nAll e2e-local tests passed.\n'
```

- [ ] **Step 2: Make executable and commit**

```bash
chmod +x test/run-e2e-local.sh
git add test/run-e2e-local.sh
git commit -m "feat(e2e): add run-e2e-local.sh pre-release runner"
```

---

## Task 7: Delete obsolete files

`binary.sh` is superseded by `apply.sh`. `install.sh` and `combined.sh` are superseded by `procure/release.sh`.

**Files:**
- Delete: `test/e2e/binary.sh`
- Delete: `test/e2e/install.sh`
- Delete: `test/e2e/combined.sh`

- [ ] **Step 1: Delete the files**

```bash
git rm test/e2e/binary.sh test/e2e/install.sh test/e2e/combined.sh
git commit -m "chore(e2e): remove binary.sh, install.sh, combined.sh (superseded)"
```

---

## Task 8: Add e2e job to ci.yml

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Append `e2e` job to `.github/workflows/ci.yml`**

The full updated file:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build
        run: go build ./...

      - name: Test
        run: go test ./...

      - name: Integration test
        run: go test -tags integration ./cmd/dotd/

  e2e:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Pre-release e2e
        run: ./test/run-e2e-local.sh
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add pre-release e2e job to CI"
```

---

## Task 9: Validate

- [ ] **Step 1: Run pre-release e2e locally**

```bash
./test/run-e2e-local.sh
```

Expected output ends with:
```
All e2e-local tests passed.
```

Each test should print `PASS: <name> test` before that final line. If any test fails it prints `FAIL: ...` and exits non-zero — the runner stops immediately.

- [ ] **Step 2: Verify no stale files remain**

```bash
ls test/e2e/
```

Expected: `apply.sh  bin.sh  check.sh  conflict.sh  context.sh  dag-order.sh  disable.sh  Dockerfile  Dockerfile.local  dry-run.sh  fixture/  idempotent.sh  list.sh  packages.sh  procure/  symlinks-nested.sh`

`binary.sh`, `install.sh`, and `combined.sh` must NOT appear.

- [ ] **Step 3: Verify no stray binary in build context**

```bash
ls test/e2e/dotd 2>/dev/null && echo 'FAIL: dotd binary not cleaned up' || echo 'OK: dotd binary cleaned up'
```

Expected: `OK: dotd binary cleaned up`

The `trap` in `run-e2e-local.sh` removes it on exit. If it's present, the trap didn't fire — investigate.
