# E2E Docker Testing Design

**Date:** 2026-05-25

## Overview

Multi-container e2e testing for `dotd` across real OS environments. Three independent tests, each in a fresh container, triggered on release.

## Test Scenarios

| Test | Starting state | Steps | Assertions |
|------|---------------|-------|------------|
| `binary` | Fresh Ubuntu container, release artifact downloaded and extracted | Run `dotd apply` against fixture | Symlinks created, `init.sh` contains expected scripts in order |
| `installer` | Fresh Ubuntu container, no binary | Run `install.sh` | Binary exists at `~/.local/bin/dotd`, `dotd --version` exits 0 |
| `combined` | Fresh Ubuntu container, no binary | Run `install.sh`, then `dotd apply` against fixture | Binary installed correctly AND symlinks/init.sh correct |

Each test runs in its own container — state never carries over between tests.

All three tests exercise the same release artifact. The binary test downloads and extracts it directly (bypassing `install.sh`); the installer and combined tests go through `install.sh`. This ensures all three test the same binary and that differences in outcome are attributable to the test path, not the binary.

## File Layout

```
test/
  e2e/
    Dockerfile          ← ubuntu:24.04 with curl explicitly installed
    fixture/            ← minimal dedicated e2e fixture (independent of unit test testdata)
    binary.sh           ← runs inside container: extract release artifact → apply → assert
    install.sh          ← runs inside container: run install.sh → assert binary present
    combined.sh         ← runs inside container: install.sh → apply → assert both
  run-e2e.sh            ← host-side: builds image, runs 3 containers
```

`.github/workflows/release.yml` calls `./test/run-e2e.sh` after the release is published.

## Fixtures

A dedicated `test/e2e/fixture/` directory — a minimal dotfiles repo with just enough structure to exercise symlinks, init.sh generation, and predicate filtering. Kept separate from `cmd/dotd/testdata/dotfiles` so unit test changes don't silently affect e2e assertions.

Minimum fixture: `env.yaml`, one `shellrc/` script, one `conf/` file, one `@when os=linux` conditional script.

**Expected assertions (binary + combined):**
- Symlink `~/.zshrc` → fixture `conf/dot-zshrc`
- `init.sh` contains the unconditional script
- `init.sh` contains the linux-gated script
- `init.sh` does not contain any macos-gated script

**Expected assertions (installer + combined):**
- `~/.local/bin/dotd` exists and is executable
- `dotd --version` exits 0 and prints the expected release tag

## CI Trigger

New `e2e` job in `.github/workflows/release.yml`:
- Runs **after** the release publish step completes (so the artifact exists on GitHub)
- Matrix: `distro: [ubuntu]` — structured for easy Fedora/macOS expansion later
- Passes the release tag to containers via environment variable
- No binary build step in CI — all three containers pull the release artifact directly

## Dockerfile

Explicitly installs curl rather than assuming it is present in the base image:

```dockerfile
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
```

`tar` and `sha256sum` are present in `ubuntu:24.04` by default.

## Distro Expansion

To add Fedora: add `fedora` to the matrix and a `test/e2e/Dockerfile.fedora`. The test scripts are distro-agnostic shell — no changes needed there.
