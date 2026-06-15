# nfpms deb/rpm Packages with GPG Signing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship GPG-signed `.deb` and `.rpm` packages as GoReleaser release artifacts so `dotd` installs via `apt install ./file` / `dnf install ./file`.

**Architecture:** Add an `nfpms` block to `.goreleaser/dotd.yaml` that packages the existing linux build into deb+rpm, signs both natively (nfpm signs in Go from an armored key file — no gpg-agent), attaches the public key to releases, and wires the private key + passphrase into the existing `_release.yml` GoReleaser step. Build packages first *unsigned*, then layer signing on top, so failures isolate.

**Tech Stack:** GoReleaser v2 (`nfpms`), nfpm native signing, GPG (RSA 4096), GitHub Actions reusable workflow (`secrets: inherit`).

**Spec:** `docs/superpowers/specs/2026-06-15-nfpms-deb-rpm-design.md`

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `.goreleaser/dotd.yaml` | Modify | Add `nfpms` block + signing + `release.extra_files`; remove dead windows override |
| `dotd-signing-key.asc` (repo root) | Create | Public signing key, committed + attached to releases |
| `.github/workflows/_release.yml` | Modify | Write private key to temp file; pass `GPG_KEY_PATH` + `NFPM_DEFAULT_PASSPHRASE` to GoReleaser |

**Note on "tests":** this is release-infra config, not Go code — there are no unit tests. The verification loop is: `goreleaser check` (config validity) → `goreleaser release --snapshot --clean` (artifacts produced) → `rpm -K` (signature present). Each task ends by running the relevant check and committing.

---

## Task 0: Install GoReleaser locally (tooling prerequisite)

GoReleaser is not installed locally (rpm + gpg are). Needed to validate config and build snapshot packages.

**Files:** none.

- [ ] **Step 1: Install GoReleaser v2**

Run:
```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

- [ ] **Step 2: Verify it is on PATH**

Run: `goreleaser --version`
Expected: prints a `GitVersion: 2.x` line. If "command not found", ensure `$(go env GOPATH)/bin` is on `$PATH` (same fix used for golangci-lint: `export PATH="$HOME/go/bin:$PATH"`).

- [ ] **Step 3: Baseline — current config is valid before any change**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid` (1 config checked). This confirms the toolchain works against the existing file. No commit — tooling only.

---

## Task 1: Remove the dead windows format_override

Independent cleanup. `archives.format_overrides` has a `windows`/`zip` entry but `goos` has no `windows`, so it never fires.

**Files:**
- Modify: `.goreleaser/dotd.yaml:29-31`

- [ ] **Step 1: Delete the dead override**

In `.goreleaser/dotd.yaml`, remove these three lines from the `archives` block:
```yaml
    format_overrides:
      - goos: windows
        format: zip
```
The `archives` block should end at the `format: tar.gz` line.

- [ ] **Step 2: Validate config still parses**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid`.

- [ ] **Step 3: Commit**

```bash
git add .goreleaser/dotd.yaml
git commit -m "chore: drop dead windows format_override from goreleaser archives"
```

---

## Task 2: Add the nfpms block (unsigned)

Get deb+rpm building before adding signing, so a packaging error can't hide behind a signing error.

**Files:**
- Modify: `.goreleaser/dotd.yaml` (add new top-level `nfpms:` block after the `archives:` block)

- [ ] **Step 1: Add the nfpms block**

Insert this top-level block after the `archives:` block (before `brews:`):
```yaml
nfpms:
  - id: dotd
    package_name: dotd
    ids: [dotd]
    formats:
      - deb
      - rpm
    maintainer: Rocne Scribner <rocne.ks@gmail.com>
    homepage: https://github.com/rocne/dot-dagger
    description: Dotfiles manager — env resolution, DAG, symlinks, and packages
    license: MIT
    bindir: /usr/bin
```
Notes: `ids` references the build id (matching `brews`), NOT `builds`. No `contents:` — GoReleaser auto-installs the build binary at `bindir`. nfpm emits packages only for the linux artifacts.

- [ ] **Step 2: Validate config**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid`.

- [ ] **Step 3: Build a snapshot and confirm packages are produced**

Run: `goreleaser release --config .goreleaser/dotd.yaml --snapshot --clean --skip=publish,sign`
Expected: completes successfully. Then:
```bash
ls dist/*.deb dist/*.rpm
```
Expected: 4 files — `dotd_*_amd64.deb`, `dotd_*_arm64.deb`, `dotd-*.x86_64.rpm`, `dotd-*.aarch64.rpm`. (rpm uses `x86_64`/`aarch64`; deb uses `amd64`/`arm64` — this is expected, not a bug.)

- [ ] **Step 4: Spot-check package contents**

Run: `rpm -qlp dist/dotd-*.x86_64.rpm`
Expected: lists `/usr/bin/dotd` (and nothing duplicated).

- [ ] **Step 5: Commit**

```bash
git add .goreleaser/dotd.yaml
git commit -m "feat: build deb and rpm packages via goreleaser nfpms"
```

---

## Task 3: Generate the signing key, commit the public key, set secrets (manual maintainer task)

One-time. Produces the committed public key and the two repo secrets the CI build needs. Per the spec's first-release runbook, all three must be in place before the next release fires.

**Files:**
- Create: `dotd-signing-key.asc` (repo root)

- [ ] **Step 1: Generate an RSA 4096, non-expiring signing key**

Use a **dedicated signing identity** (`dotd-packaging@users.noreply.github.com`) — NOT your personal `rocne.ks@gmail.com`. A distinct UID guarantees every `gpg` lookup below is unambiguous even if you already have a personal key, and capturing the fingerprint makes it airtight.

Run (batch, non-interactive):
```bash
cat > /tmp/dotd-key.conf <<'EOF'
%no-protection
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: dotd package signing
Name-Email: dotd-packaging@users.noreply.github.com
Expire-Date: 0
%commit
EOF
gpg --batch --generate-key /tmp/dotd-key.conf
FPR=$(gpg --list-keys --with-colons dotd-packaging@users.noreply.github.com | awk -F: '/^fpr:/{print $10; exit}')
echo "Signing key fingerprint: $FPR"
```
Expected: a new key is created and `$FPR` prints a 40-char fingerprint. `Expire-Date: 0` = non-expiring (spec requirement — an expiry silently breaks `rpm -K` on shipped packages). `%no-protection` creates a passphrase-less key for simplest CI; if you prefer a passphrase, drop that line, set one, and store it as `GPG_PASSPHRASE` (Step 4). Decide now — passphrase-less means `GPG_PASSPHRASE`/`NFPM_DEFAULT_PASSPHRASE` are omitted everywhere below. **Keep `$FPR` exported in this shell** — Steps 2-3 and Tasks 4/7 reference it.

- [ ] **Step 2: Export the public key to the repo root**

Run:
```bash
gpg --armor --export "$FPR" > dotd-signing-key.asc
```
Expected: `dotd-signing-key.asc` exists at repo root and begins `-----BEGIN PGP PUBLIC KEY BLOCK-----`.

- [ ] **Step 3: Export the private key (for the GitHub secret)**

Run:
```bash
gpg --armor --export-secret-keys "$FPR" > /tmp/dotd-private.asc
```
Expected: armored private key written. This file is for the secret only — do **not** commit it.

- [ ] **Step 4: Set repo secrets**

Run:
```bash
gh secret set GPG_PRIVATE_KEY < /tmp/dotd-private.asc
# Only if the key has a passphrase (Step 1):
# printf '%s' '<passphrase>' | gh secret set GPG_PASSPHRASE
```
Expected: `✓ Set secret GPG_PRIVATE_KEY`. Then scrub the temp private key: `shred -u /tmp/dotd-private.asc /tmp/dotd-key.conf` (or `rm -P` / `rm`).

- [ ] **Step 5: Commit the public key**

```bash
git add dotd-signing-key.asc
git commit -m "chore: add dotd package signing public key"
```

---

## Task 4: Add signing to the nfpms block

Layer signing onto the now-working packages and verify the signature locally.

**Files:**
- Modify: `.goreleaser/dotd.yaml` (the `nfpms` block from Task 2)

- [ ] **Step 1: Add rpm + deb signature config**

Add these two keys under the `nfpms` list item (same level as `bindir`):
```yaml
    rpm:
      signature:
        key_file: "{{ .Env.GPG_KEY_PATH }}"
    deb:
      signature:
        key_file: "{{ .Env.GPG_KEY_PATH }}"
```

- [ ] **Step 2: Validate config**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid`.

- [ ] **Step 3: Point the key env at the local private key and build**

The signing key is on this machine from Task 3 (reuse `$FPR`, or re-derive it). Export the private key to the path GoReleaser expects, then snapshot-build. `--skip=sign` skips the **cosign** `signs` block (cosign is not installed locally) — it does **not** affect nfpm's native package signing, which is inline in the nfpms build:
```bash
FPR=$(gpg --list-keys --with-colons dotd-packaging@users.noreply.github.com | awk -F: '/^fpr:/{print $10; exit}')
gpg --armor --export-secret-keys "$FPR" > /tmp/dotd-signing.asc
export GPG_KEY_PATH=/tmp/dotd-signing.asc
export VERSION=0.0.0-dev   # ldflags reference {{.Env.VERSION}}; required for local snapshot
# If the key has a passphrase: export NFPM_DEFAULT_PASSPHRASE='<passphrase>'
goreleaser release --config .goreleaser/dotd.yaml --snapshot --clean --skip=publish,sign
```
Expected: build succeeds; `dist/` repopulates with deb+rpm.

- [ ] **Step 4: Verify the rpm signature**

Run:
```bash
rpm --import dotd-signing-key.asc
rpm -K dist/*.rpm
```
Expected: each line ends with `digests signatures OK` (or `pgp ... OK`), not `NOKEY`/`SIGNATURES NOT OK`. Then clean up: `shred -u /tmp/dotd-signing.asc` (or `rm`).

- [ ] **Step 5: Commit**

```bash
git add .goreleaser/dotd.yaml
git commit -m "feat: GPG-sign deb and rpm packages"
```

---

## Task 5: Attach the public key to every release

**Files:**
- Modify: `.goreleaser/dotd.yaml` (add a top-level `release:` block)

- [ ] **Step 1: Add release.extra_files**

Add this top-level block (e.g. after `checksum:`):
```yaml
release:
  extra_files:
    - glob: ./dotd-signing-key.asc
```

- [ ] **Step 2: Validate config**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid`. (Full attachment only happens on a real release; config validity + the file existing at repo root is the check we can run locally. The file was committed in Task 3.)

- [ ] **Step 3: Confirm the referenced file exists**

Run: `test -f dotd-signing-key.asc && echo present`
Expected: `present`.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser/dotd.yaml
git commit -m "feat: attach package signing public key to releases"
```

---

## Task 6: Wire the private key into the release workflow

**Files:**
- Modify: `.github/workflows/_release.yml:47-59`

- [ ] **Step 1: Add a key-preparation step before the Release step**

In `.github/workflows/_release.yml`, immediately before the `- name: Release` step (currently line 49), insert:
```yaml
      - name: Prepare package signing key
        env:
          GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
        run: |
          umask 077
          printf '%s' "$GPG_PRIVATE_KEY" > "$RUNNER_TEMP/dotd-signing-key.asc"
          echo "GPG_KEY_PATH=$RUNNER_TEMP/dotd-signing-key.asc" >> "$GITHUB_ENV"
```
This writes the armored private key to a temp file with restrictive perms and exposes its path as `GPG_KEY_PATH` to later steps. No `gpg --import` — nfpm reads the file directly.

- [ ] **Step 2: Pass the signing env to the Release step**

In the `- name: Release` step's `env:` block (currently lines 55-59), add:
```yaml
          GPG_KEY_PATH: ${{ env.GPG_KEY_PATH }}
          NFPM_DEFAULT_PASSPHRASE: ${{ secrets.GPG_PASSPHRASE }}
```
If the key is passphrase-less (Task 3, `%no-protection`), omit the `NFPM_DEFAULT_PASSPHRASE` line — `GPG_PASSPHRASE` won't exist as a secret.

- [ ] **Step 3: Lint the workflow YAML**

Run: `actionlint .github/workflows/_release.yml 2>/dev/null || python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/_release.yml')); print('yaml ok')"`
Expected: `actionlint` clean, or `yaml ok` if actionlint isn't installed.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/_release.yml
git commit -m "ci: supply package signing key to goreleaser nfpms"
```

---

## Task 7: Final validation, push, PR

**Files:** none (verification + PR).

- [ ] **Step 1: Full clean snapshot build from scratch**

Run:
```bash
FPR=$(gpg --list-keys --with-colons dotd-packaging@users.noreply.github.com | awk -F: '/^fpr:/{print $10; exit}')
gpg --armor --export-secret-keys "$FPR" > /tmp/dotd-signing.asc
export GPG_KEY_PATH=/tmp/dotd-signing.asc
export VERSION=0.0.0-dev   # ldflags reference {{.Env.VERSION}}; required for local snapshot
goreleaser release --config .goreleaser/dotd.yaml --snapshot --clean --skip=publish,sign
rpm -K dist/*.rpm
ls dist/*.deb dist/*.rpm
shred -u /tmp/dotd-signing.asc 2>/dev/null || rm -f /tmp/dotd-signing.asc
```
Expected: 2 deb + 2 rpm, all rpm signatures OK.

- [ ] **Step 2: Confirm the existing pipeline still validates**

Run: `goreleaser check --config .goreleaser/dotd.yaml`
Expected: `config is valid`.

- [ ] **Step 3: Push and open PR**

Run:
```bash
git push -u origin feature/claude-nfpms-deb-rpm
gh pr create --fill --title "feat: ship GPG-signed deb and rpm packages"
```
PR body should note: first release after merge requires `GPG_PRIVATE_KEY` (and `GPG_PASSPHRASE` if used) secrets set — done in Task 3 — and that `dotd-signing-key.asc` ships at repo root.

- [ ] **Step 4: Watch CI — but know its limit**

Run: `gh pr checks --watch`
Expected: all checks pass (lint, build, test).

**Validation gap — read this:** PR CI does **not** run GoReleaser, so it does **not** build or sign packages. The only pre-merge validation of package signing is the **local snapshot in Step 1**. The first *real* signed `.deb`/`.rpm` build happens when the next release PR merges and `_release.yml` runs. Watch that release run closely (`gh run watch`); if `GPG_KEY_PATH` or the passphrase is wrong, the GoReleaser step fails loud there, not in this PR.

---

## Self-Review notes (author)

- **Spec coverage:** nfpms block (T2), GPG signing both formats (T4), public key dist committed+attached (T3,T5), CI wiring (T6), windows cleanup (T1) — all mapped. First-release runbook → T3 (key/secrets/pubkey) executed before merge.
- **Field-name correction vs spec:** spec wrote `builds: [dotd]`; GoReleaser nfpms uses **`ids: [dotd]`** (verified against the existing `brews` block, which uses `ids`). Plan uses `ids`.
- **Passphrase branch:** plan offers passphrase-less (`%no-protection`) as the simplest path and notes exactly which lines to drop. Either branch is internally consistent.
- **Snapshot-signing assumption:** T4 Step 3 carries the fallback if cosign signing complains in snapshot (`--skip=sign` does not affect nfpm signing). This is the spec's "validate first" item, resolved in-task.
