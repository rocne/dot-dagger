# Pre-Release E2E Docker Testing

**Date:** 2026-05-27
**Branch:** feature/claude-initgen-homedir-cleanup (or new branch)
**Status:** Approved for implementation

## Problem

Post-release e2e tests only run after a GitHub release is published. Binary behavior bugs are caught too late — after the tag is pushed, the release is live, and the build is out.

## Goal

Run the same Docker e2e tests before release using a locally-built binary. Catch binary behavior bugs on every PR.

## Mental Model

Two roles, strictly separated:

- **Procurer** — gets `dotd` onto PATH inside a running container
- **Exerciser** — test script that calls `dotd`, asserts behavior

Procurer and exerciser run sequentially in the same container. Each exerciser test gets a fresh container (isolation). Procurer re-runs per container.

## Binary Procurers

### Release procurer: `test/e2e/procure/release.sh`
Runs the real `install.sh --version $TAG` inside the container. Downloads from GitHub. This is the installer test — it verifies the installer works end-to-end on a live system.

### Local procurer: `test/e2e/procure/local.sh`
Copies the pre-built binary from `/staged/dotd` (baked into the image) to `~/.local/bin/dotd` and makes it executable. Mimics the install path so exercisers are identical.

## Docker Images

### `test/e2e/Dockerfile` (existing, unchanged)
Ubuntu 24.04 base with curl and ca-certs. Used for post-release testing.

### `test/e2e/Dockerfile.local` (new)
Standalone image (does not depend on `dotd-e2e` being pre-built). Duplicates the base setup and adds the staged binary:
```dockerfile
FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY binary.sh install.sh combined.sh context.sh dag-order.sh \
     dry-run.sh idempotent.sh check.sh list.sh bin.sh \
     symlinks-nested.sh disable.sh packages.sh conflict.sh /tests/
COPY procure/ /procure/
COPY dotd /staged/dotd
```
Used for pre-release testing. Binary is cross-compiled for linux/amd64 and placed in the build context before `docker build`.

## Exerciser Scripts

All existing test scripts (`context.sh`, `dag-order.sh`, `dry-run.sh`, `idempotent.sh`, `check.sh`, `list.sh`, `bin.sh`, `symlinks-nested.sh`, `disable.sh`, `packages.sh`, `conflict.sh`) are refactored, and `binary.sh` is renamed to `apply.sh`:

- Remove the binary download/install step
- Call `dotd` directly (binary is on PATH after procurer runs)
- `DOTD_VERSION` env var only needed by release procurer, not exercisers

The existing `install.sh` test is replaced by `procure/release.sh` — the installer logic moves into the procurer. Any installer-specific assertions (e.g. `test -x ~/.local/bin/dotd`) live in the release procurer.

The existing `combined.sh` test is removed. Its value (install + apply in one flow) is fully covered by running `procure/release.sh` + the exerciser scripts.

## Runner Scripts

### `test/run-e2e.sh` (refactored)
Post-release runner. Requires `DOTD_VERSION`. Uses `dotd-e2e` image.

Per test:
```sh
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${FIXTURE}:/fixture:ro" \
  -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
  dotd-e2e \
  sh -c ". /procure/release.sh && sh /tests/EXERCISER.sh"
```

Every test gets the `install.sh` volume mount because every test now runs the release procurer (which calls `/repo/install.sh`). Previously only the `install.sh` and `combined.sh` tests received this mount.

### `test/run-e2e-local.sh` (new)
Pre-release runner. No version required.

Steps:
1. Cross-compile: `GOOS=linux GOARCH=amd64 go build -o test/e2e/dotd ./cmd/dotd`
2. Register cleanup with `trap 'rm -f test/e2e/dotd' EXIT` so the binary is removed even if the build fails
3. Build image: `docker build -t dotd-e2e-local -f test/e2e/Dockerfile.local test/e2e/`
4. Per test:
```sh
docker run --rm \
  -v "${FIXTURE}:/fixture:ro" \
  dotd-e2e-local \
  sh -c ". /procure/local.sh && sh /tests/EXERCISER.sh"
```

Both runners run the same exerciser list in the same order.

## CI Changes

### `ci.yml` — new job `e2e`

```yaml
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

Runs on every PR and push to main, after the `test` job passes.

## PATH Consistency

Both procurers install `dotd` to `~/.local/bin/dotd`. Each procurer script ends with `export PATH="$HOME/.local/bin:$PATH"`. The runner sources the procurer with `.` (not `sh`) so the export survives into the exerciser:

```sh
sh -c ". /procure/PROCURER.sh && sh /tests/EXERCISER.sh"
```

Using `sh /procure/...` would spawn a subshell — the `export` would die when it exits and the exerciser would not find `dotd` on PATH.

## File Changes Summary

| File | Change |
|------|--------|
| `test/e2e/Dockerfile` | Unchanged |
| `test/e2e/Dockerfile.local` | New |
| `test/e2e/procure/release.sh` | New — installer test + PATH setup |
| `test/e2e/procure/local.sh` | New — copy staged binary + PATH setup |
| `test/e2e/binary.sh` | Renamed to `apply.sh`, drop download step |
| `test/e2e/install.sh` | Removed — superseded by `procure/release.sh` |
| `test/e2e/combined.sh` | Removed |
| All other `test/e2e/*.sh` | Refactored — drop download step |
| `test/run-e2e.sh` | Refactored — use procurer pattern |
| `test/run-e2e-local.sh` | New |
| `.github/workflows/ci.yml` | Add `e2e` job |
