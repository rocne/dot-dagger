# E2E Docker Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three Docker-based e2e tests (binary, installer, combined) that run against a real Ubuntu container after each release.

**Architecture:** Each test runs in its own fresh container from a shared `ubuntu:24.04` image. The binary test downloads the release artifact directly via curl. The installer and combined tests run `install.sh` against the published release. A host-side `test/run-e2e.sh` script orchestrates all three. A new `e2e` job in `release.yml` triggers after the release job completes.

**Tech Stack:** POSIX shell, Docker, GitHub Actions, `ubuntu:24.04`

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `test/e2e/fixture/env.yaml` | Create | Minimal env context for fixture |
| `test/e2e/fixture/conf/dot-zshrc` | Create | Config file for symlink assertion |
| `test/e2e/fixture/shellrc/base.sh` | Create | Unconditional script (always in init.sh) |
| `test/e2e/fixture/shellrc/linux.sh` | Create | Linux-gated script (in init.sh when os=linux) |
| `test/e2e/fixture/shellrc/macos.sh` | Create | macOS-gated script (never in init.sh when os=linux) |
| `test/e2e/Dockerfile` | Create | Ubuntu base image with curl installed |
| `test/e2e/binary.sh` | Create | In-container: extract release artifact → apply → assert |
| `test/e2e/install.sh` | Create | In-container: run install.sh → assert binary present |
| `test/e2e/combined.sh` | Create | In-container: install.sh → apply → assert both |
| `test/run-e2e.sh` | Create | Host-side: build image, run 3 containers |
| `.github/workflows/release.yml` | Modify | Add `e2e` job after `release` job |

---

### Task 1: E2E fixture

**Files:**
- Create: `test/e2e/fixture/env.yaml`
- Create: `test/e2e/fixture/conf/dot-zshrc`
- Create: `test/e2e/fixture/shellrc/base.sh`
- Create: `test/e2e/fixture/shellrc/linux.sh`
- Create: `test/e2e/fixture/shellrc/macos.sh`

The fixture is a minimal dotfiles repo. `env.yaml` sets `context: personal`. `conf/dot-zshrc` is an empty placeholder — its presence is enough to assert symlink creation. The three shellrc scripts exercise conditional inclusion: `base.sh` is always active, `linux.sh` is gated on `os=linux`, `macos.sh` is gated on `os=macos` and must be absent from init.sh when running linux tests.

- [ ] **Step 1: Create fixture directory and env.yaml**

```sh
mkdir -p test/e2e/fixture/conf test/e2e/fixture/shellrc
```

`test/e2e/fixture/env.yaml`:
```yaml
context: personal
```

- [ ] **Step 2: Create conf/dot-zshrc**

`test/e2e/fixture/conf/dot-zshrc`:
```sh
# zshrc placeholder
```

- [ ] **Step 3: Create shellrc scripts**

`test/e2e/fixture/shellrc/base.sh`:
```bash
#!/bin/bash
# base environment — always active
export DOT_BASE_LOADED=1
```

`test/e2e/fixture/shellrc/linux.sh`:
```bash
#!/bin/bash
# @when(os=linux)
# @after(shellrc.base)
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
```

`test/e2e/fixture/shellrc/macos.sh`:
```bash
#!/bin/bash
# @when(os=macos)
# @after(shellrc.base)
export HOMEBREW_PREFIX="/opt/homebrew"
```

- [ ] **Step 4: Commit**

```bash
git add test/e2e/fixture/
git commit -m "test(e2e): add minimal fixture for e2e tests"
```

---

### Task 2: Dockerfile

**Files:**
- Create: `test/e2e/Dockerfile`

The image is based on `ubuntu:24.04`. It explicitly installs `curl` and `ca-certificates` (do not assume they are present in the minimal base image). The three test scripts are copied in at build time so a single `docker build` produces a self-contained image.

- [ ] **Step 1: Create Dockerfile**

`test/e2e/Dockerfile`:
```dockerfile
FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY binary.sh /tests/binary.sh
COPY install.sh /tests/install.sh
COPY combined.sh /tests/combined.sh
```

- [ ] **Step 2: Verify syntax**

```bash
docker build -t dotd-e2e-check test/e2e/
```

Expected: image builds successfully. (The test scripts don't exist yet — this step will fail at the COPY lines. Skip and proceed once scripts are written in Tasks 3–5, then re-run.)

- [ ] **Step 3: Commit**

```bash
git add test/e2e/Dockerfile
git commit -m "test(e2e): add Ubuntu Dockerfile for e2e tests"
```

---

### Task 3: binary.sh

**Files:**
- Create: `test/e2e/binary.sh`

Runs inside the container. Downloads the release tarball directly via curl (same artifact as `install.sh` would fetch, bypassing the installer path). Extracts the binary, runs `dotd apply` against the mounted fixture, then asserts symlinks and init.sh contents.

`DOTD_VERSION` is passed in as an environment variable by the host runner. The release asset naming convention is `dotd_<TAG>_linux_amd64.tar.gz` (matches GoReleaser output and install.sh logic).

- [ ] **Step 1: Create binary.sh**

`test/e2e/binary.sh`:
```sh
#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

# download and extract release artifact
curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

# create isolated home dirs
mkdir -p /home/e2e/bin /tmp/generated

# apply against fixture
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux

# assertions
test -L /home/e2e/.zshrc          || { printf 'FAIL: .zshrc symlink missing\n'; exit 1; }
grep -q "base.sh"  /tmp/init.sh    || { printf 'FAIL: base.sh not in init.sh\n'; exit 1; }
grep -q "linux.sh" /tmp/init.sh    || { printf 'FAIL: linux.sh not in init.sh\n'; exit 1; }
! grep -q "macos.sh" /tmp/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n'; exit 1; }

printf 'PASS: binary test\n'
```

- [ ] **Step 2: Check syntax**

```bash
sh -n test/e2e/binary.sh
```

Expected: no output (no syntax errors).

- [ ] **Step 3: Commit**

```bash
git add test/e2e/binary.sh
git commit -m "test(e2e): add binary test script"
```

---

### Task 4: install.sh (test script)

**Files:**
- Create: `test/e2e/install.sh`

Runs inside the container. Mounts the repo's `install.sh` at `/repo/install.sh`. Calls it with `--version $DOTD_VERSION` to install the specific release. Asserts the binary is present and executable, and that `dotd --version` exits 0.

Note: this file is named `install.sh` but lives at `test/e2e/install.sh` — it is the *test* for the installer, not the installer itself.

- [ ] **Step 1: Create test/e2e/install.sh**

`test/e2e/install.sh`:
```sh
#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

# run the project installer for this specific release
sh /repo/install.sh --version "${TAG}"

# assertions
test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed at ~/.local/bin/dotd\n'; exit 1; }
"${HOME}/.local/bin/dotd" --version || { printf 'FAIL: dotd --version exited non-zero\n'; exit 1; }

printf 'PASS: installer test\n'
```

- [ ] **Step 2: Check syntax**

```bash
sh -n test/e2e/install.sh
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add test/e2e/install.sh
git commit -m "test(e2e): add installer test script"
```

---

### Task 5: combined.sh

**Files:**
- Create: `test/e2e/combined.sh`

Runs inside the container. Installs via `install.sh`, then runs `dotd apply` using the just-installed binary. Asserts both the installer outcome and the apply outcome. This is the only test that exercises the full path a real user would take.

- [ ] **Step 1: Create combined.sh**

`test/e2e/combined.sh`:
```sh
#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

# install via install.sh
sh /repo/install.sh --version "${TAG}"

# assert binary present before proceeding
test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed\n'; exit 1; }

# create isolated home dirs
mkdir -p /home/e2e/bin /tmp/generated

# apply against fixture using the installed binary
"${HOME}/.local/bin/dotd" apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux

# assertions
test -L /home/e2e/.zshrc          || { printf 'FAIL: .zshrc symlink missing\n'; exit 1; }
grep -q "base.sh"  /tmp/init.sh    || { printf 'FAIL: base.sh not in init.sh\n'; exit 1; }
grep -q "linux.sh" /tmp/init.sh    || { printf 'FAIL: linux.sh not in init.sh\n'; exit 1; }
! grep -q "macos.sh" /tmp/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n'; exit 1; }

printf 'PASS: combined test\n'
```

- [ ] **Step 2: Check syntax**

```bash
sh -n test/e2e/combined.sh
```

Expected: no output.

- [ ] **Step 3: Build the image now that all scripts exist**

```bash
docker build -t dotd-e2e test/e2e/
```

Expected: image builds successfully, all three COPY steps resolve.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/combined.sh
git commit -m "test(e2e): add combined test script"
```

---

### Task 6: run-e2e.sh

**Files:**
- Create: `test/run-e2e.sh`

Host-side orchestrator. Builds the image once, then runs all three containers in sequence. Accepts `DOTD_VERSION` from the environment; if unset, fetches the latest release tag from the GitHub API. Mounts the fixture and `install.sh` as read-only volumes at runtime.

- [ ] **Step 1: Create test/run-e2e.sh**

`test/run-e2e.sh`:
```sh
#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# resolve tag
if [ -z "${DOTD_VERSION}" ]; then
  DOTD_VERSION="$(curl -fsSL "https://api.github.com/repos/rocne/dot-dagger/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

printf 'e2e: testing release %s\n' "${DOTD_VERSION}"

# build image
docker build -t dotd-e2e "${SCRIPT_DIR}/e2e"

# binary test
printf '\n=== binary test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/binary.sh

# installer test
printf '\n=== installer test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
  dotd-e2e sh /tests/install.sh

# combined test
printf '\n=== combined test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/combined.sh

printf '\nAll e2e tests passed.\n'
```

- [ ] **Step 2: Make executable and check syntax**

```bash
chmod +x test/run-e2e.sh
sh -n test/run-e2e.sh
```

Expected: no syntax errors.

- [ ] **Step 3: Smoke-test locally (optional but recommended)**

```bash
DOTD_VERSION=v0.2.32 ./test/run-e2e.sh
```

Expected: all three tests print `PASS`. Requires Docker and network access.

- [ ] **Step 4: Commit**

```bash
git add test/run-e2e.sh
git commit -m "test(e2e): add host-side e2e runner script"
```

---

### Task 7: Add e2e job to release.yml

**Files:**
- Modify: `.github/workflows/release.yml`

Add an `e2e` job that depends on `release`. Uses a matrix over `distro: [ubuntu]` for future expansion. Passes `DOTD_VERSION` from the git tag (`github.ref_name`). Docker is pre-installed on `ubuntu-latest` GHA runners — no extra setup needed.

- [ ] **Step 1: Add e2e job to release.yml**

Append to `.github/workflows/release.yml` after the `release` job:

```yaml
  e2e:
    needs: release
    runs-on: ubuntu-latest
    strategy:
      matrix:
        distro: [ubuntu]
    steps:
      - uses: actions/checkout@v4

      - name: Run e2e tests (${{ matrix.distro }})
        run: ./test/run-e2e.sh
        env:
          DOTD_VERSION: ${{ github.ref_name }}
```

- [ ] **Step 2: Verify the full release.yml is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit and push**

```bash
git add .github/workflows/release.yml
git commit -m "ci(release): add e2e job — runs after release, matrix over distro"
git push
```

- [ ] **Step 4: Open PR**

```bash
gh pr create \
  --title "feat(e2e): Docker-based e2e tests for install and apply" \
  --body "Three-container e2e suite triggered on release. Tests binary extraction, install.sh, and combined install+apply on Ubuntu. No new tooling required — curl only."
```
