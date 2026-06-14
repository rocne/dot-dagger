# Release Supply-Chain Hardening (cosign + SLSA) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sign release artifacts with keyless cosign and emit SLSA build provenance, enforce verification in `install.sh`, and document it — with zero disruption to the GoReleaser / release-please / Homebrew flow.

**Architecture:** GoReleaser gains a `signs:` block that keyless-signs the checksums file. `_release.yml` installs cosign, attests the archives via `actions/attest-build-provenance`, and (in its `e2e` job) verifies the published signature + provenance as a smoke test. Both release callers grant the `id-token`/`attestations` permissions. `install.sh` verifies the checksums-file signature when cosign is present, aborting only on a genuine bad signature. README documents manual verification.

**Tech Stack:** GoReleaser v2, Sigstore cosign (keyless OIDC), GitHub `actions/attest-build-provenance@v2`, POSIX `sh`.

**Spec:** `docs/superpowers/specs/2026-06-14-cosign-slsa-design.md`

**Testing note (read first):** This is release *infrastructure* — YAML config and a POSIX install script. The repo has no unit-test harness for these (install.sh is untested by convention), and the real signing path needs CI's OIDC token, so it cannot run on a laptop. Each task is therefore validated by the strongest *static* check available locally (config parse, workflow lint, `sh -n`) plus targeted `grep` assertions; the authoritative end-to-end validation is Task 8, a manual checklist run against the first real release after merge. Local tools are invoked via `go run …@<pinned>` because none are installed and this is a Go repo. If a `go run` fetch is blocked by no network, fall back to the noted parser-only check and rely on CI.

All work happens on branch `feature/claude-cosign-slsa` (the spec is already committed there).

---

### Task 1: GoReleaser `signs:` block

**Files:**
- Modify: `.goreleaser/dotd.yaml` (append after the `checksum:` block, currently lines 48-49)

- [ ] **Step 1: Add the `signs:` block**

Append to `.goreleaser/dotd.yaml`, after the `checksum:` block and before `changelog:`:

```yaml
signs:
  - cmd: cosign
    artifacts: checksum
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-certificate=${certificate}"
      - "--output-signature=${signature}"
      - "${artifact}"
      - "--yes"
```

- [ ] **Step 2: Validate the GoReleaser config parses**

Run: `go run github.com/goreleaser/goreleaser/v2@v2.4.4 check -f .goreleaser/dotd.yaml`
Expected: `config is valid` (warnings about deprecations are fine; an error about the `signs` schema is not).

Fallback if the module fetch is blocked: `python3 -c "import yaml; yaml.safe_load(open('.goreleaser/dotd.yaml')); print('yaml ok')"` → `yaml ok`.

- [ ] **Step 3: Assert the block is present**

Run: `grep -A1 '^signs:' .goreleaser/dotd.yaml`
Expected: shows `- cmd: cosign`.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser/dotd.yaml
git commit -m "ci: keyless cosign-sign the release checksums file"
```

---

### Task 2: Grant `id-token` + `attestations` permissions in all three workflows

Reusable workflows receive only the permissions their caller grants, capped by their own `permissions:`. So the two callers AND the callee must list the new permissions, or keyless signing (`id-token`) and provenance writes (`attestations`) fail.

**Files:**
- Modify: `.github/workflows/_release.yml` (top-level `permissions:`, currently lines 21-23)
- Modify: `.github/workflows/release-please.yml` (`release` job `permissions:`, currently lines 36-38)
- Modify: `.github/workflows/release.yml` (`release` job `permissions:`, currently lines 17-19)

- [ ] **Step 1: `_release.yml` — extend top-level permissions**

Replace:

```yaml
permissions:
  contents: write
  issues: write
```

with:

```yaml
permissions:
  contents: write
  issues: write
  id-token: write
  attestations: write
```

- [ ] **Step 2: `release-please.yml` — extend the `release` job permissions**

In the `release` job, replace:

```yaml
    permissions:
      contents: write
      issues: write
```

with:

```yaml
    permissions:
      contents: write
      issues: write
      id-token: write
      attestations: write
```

- [ ] **Step 3: `release.yml` — extend the `release` job permissions**

Apply the identical replacement as Step 2 in `.github/workflows/release.yml`'s `release` job.

- [ ] **Step 4: Lint the workflows**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/_release.yml .github/workflows/release-please.yml .github/workflows/release.yml`
Expected: no output (exit 0).

Fallback if blocked: `for f in _release release-please release; do python3 -c "import yaml; yaml.safe_load(open('.github/workflows/$f.yml')); print('$f ok')"; done`.

- [ ] **Step 5: Assert all three files grant both permissions**

Run: `grep -l 'id-token: write' .github/workflows/_release.yml .github/workflows/release-please.yml .github/workflows/release.yml | wc -l`
Expected: `3`.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/_release.yml .github/workflows/release-please.yml .github/workflows/release.yml
git commit -m "ci: grant id-token and attestations permissions to release jobs"
```

---

### Task 3: Install cosign + attest provenance in the `release` job

**Files:**
- Modify: `.github/workflows/_release.yml` (the `release` job `steps:`, currently lines 28-47)

- [ ] **Step 1: Add the cosign-installer step before the GoReleaser step**

In the `release` job, immediately after the `actions/setup-go@v6` step and before the `Release` (goreleaser) step, insert:

```yaml
      - uses: sigstore/cosign-installer@v3
```

- [ ] **Step 2: Add the provenance attestation step after the GoReleaser step**

Immediately after the `Release` step (the `goreleaser/goreleaser-action@v7` step), append to the `release` job:

```yaml
      - name: Attest build provenance
        uses: actions/attest-build-provenance@v2
        with:
          subject-path: dist/dotd_*.tar.gz
```

(`dist/` is where GoReleaser writes archives under `--clean`; this runs in the same job/workspace.)

- [ ] **Step 3: Lint**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/_release.yml`
Expected: no output (exit 0).

- [ ] **Step 4: Assert both steps present**

Run: `grep -E 'cosign-installer|attest-build-provenance' .github/workflows/_release.yml`
Expected: both lines shown.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/_release.yml
git commit -m "ci: install cosign and attest build provenance in release job"
```

---

### Task 4: Post-release verification smoke test in the `e2e` job

The `e2e` job is a separate runner from `release`, so it needs its own cosign install. `gh` is preinstalled on GitHub runners.

**Files:**
- Modify: `.github/workflows/_release.yml` (the `e2e` job `steps:`, currently lines 55-70)

- [ ] **Step 1: Add cosign-installer + a verify step to the `e2e` job**

In the `e2e` job, immediately after the `actions/checkout@v6` step and before the `Run e2e tests` step, insert:

```yaml
      - uses: sigstore/cosign-installer@v3

      - name: Verify release signature and provenance
        run: |
          TAG="${{ inputs.version }}"
          BASE="https://github.com/${{ github.repository }}/releases/download/$TAG"
          CHECKSUMS="dotd_${TAG}_checksums.txt"
          ARCHIVE="dotd_${TAG}_linux_amd64.tar.gz"
          curl -fsSL -o "$CHECKSUMS"     "$BASE/$CHECKSUMS"
          curl -fsSL -o "$CHECKSUMS.sig" "$BASE/$CHECKSUMS.sig"
          curl -fsSL -o "$CHECKSUMS.pem" "$BASE/$CHECKSUMS.pem"
          curl -fsSL -o "$ARCHIVE"       "$BASE/$ARCHIVE"
          cosign verify-blob \
            --certificate "$CHECKSUMS.pem" \
            --signature   "$CHECKSUMS.sig" \
            --certificate-identity-regexp "^https://github\.com/${{ github.repository }}/\.github/workflows/_release\.yml@" \
            --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
            "$CHECKSUMS"
          gh attestation verify "$ARCHIVE" --repo "${{ github.repository }}"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Lint**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/_release.yml`
Expected: no output (exit 0).

- [ ] **Step 3: Assert the verify step is in the `e2e` job**

Run: `grep -n 'Verify release signature and provenance' .github/workflows/_release.yml`
Expected: one match.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/_release.yml
git commit -m "ci: verify signature and provenance as a post-release smoke test"
```

---

### Task 5: Enforce signature verification in `install.sh`

**Files:**
- Modify: `install.sh` (insert after the checksum-verify block, which currently ends at the `fi` on line ~142, before the `# --- extract and install ---` comment)

- [ ] **Step 1: Insert the signature-verification block**

Between the end of the `# --- verify checksum ---` block and the `# --- extract and install ---` line, insert:

```sh
# --- verify signature (enforced when cosign is present) ---
SIG="${CHECKSUMS}.sig"
CERT="${CHECKSUMS}.pem"
if command -v cosign >/dev/null 2>&1; then
  if curl -fsSL -o "$TMP/$SIG" "$BASE_URL/$SIG" 2>/dev/null \
    && curl -fsSL -o "$TMP/$CERT" "$BASE_URL/$CERT" 2>/dev/null; then
    printf 'verifying signature...\n'
    if cosign verify-blob \
        --certificate "$TMP/$CERT" \
        --signature   "$TMP/$SIG" \
        --certificate-identity-regexp "^https://github\.com/${REPO}/\.github/workflows/_release\.yml@" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
        "$TMP/$CHECKSUMS" >/dev/null 2>&1; then
      printf 'signature verified\n'
    else
      printf 'error: signature verification FAILED for %s — aborting\n' "$CHECKSUMS" >&2
      exit 1
    fi
  else
    printf 'notice: no signature published for %s — skipping signature verification\n' "$TAG" >&2
  fi
else
  printf 'notice: cosign not found — skipping signature verification (install cosign v2+ for full verification)\n' >&2
fi
```

(`REPO`, `BASE_URL`, `TMP`, `CHECKSUMS`, `TAG` are all already defined earlier in the script.)

- [ ] **Step 2: Syntax-check the script**

Run: `sh -n install.sh`
Expected: no output (exit 0).

- [ ] **Step 3: Branch test — cosign absent**

Run:
```bash
PATH="/usr/bin:/bin" sh -c 'command -v cosign >/dev/null 2>&1 && echo present || echo absent'
```
Expected: `absent` — confirms the no-cosign branch is reachable on a clean PATH (this is the curl-only fallback that must keep working).

- [ ] **Step 4: Branch test — cosign present, bad signature aborts**

Run:
```bash
d=$(mktemp -d)
printf '#!/bin/sh\nexit 1\n' > "$d/cosign"; chmod +x "$d/cosign"
PATH="$d:$PATH" sh -c '
  command -v cosign >/dev/null 2>&1 && \
  if cosign verify-blob x 2>/dev/null; then echo "WRONG: passed"; else echo "OK: stub fails -> abort path taken"; fi'
rm -rf "$d"
```
Expected: `OK: stub fails -> abort path taken` — confirms a non-zero cosign exit drives the abort branch.

- [ ] **Step 5: Commit**

```bash
git add install.sh
git commit -m "feat: verify release signature in install.sh when cosign is present"
```

---

### Task 6: Document verification in the README

**Files:**
- Modify: `README.md` (add a `## Verifying releases` section immediately after the `## Install` section, before `## Quick start`)

- [ ] **Step 1: Add the section**

Insert this section after the Install section:

```markdown
## Verifying releases

Release artifacts are signed with [cosign](https://docs.sigstore.dev/) (keyless,
no private keys) and carry [SLSA build provenance](https://slsa.dev/). `install.sh`
verifies the signature automatically when `cosign` is installed; you can also
verify manually.

### Signature (requires cosign v2+)

```sh
TAG=v0.6.1   # the release you downloaded
BASE="https://github.com/rocne/dot-dagger/releases/download/$TAG"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt.sig"
curl -fsSLO "$BASE/dotd_${TAG}_checksums.txt.pem"

cosign verify-blob \
  --certificate "dotd_${TAG}_checksums.txt.pem" \
  --signature   "dotd_${TAG}_checksums.txt.sig" \
  --certificate-identity-regexp "^https://github\.com/rocne/dot-dagger/\.github/workflows/_release\.yml@" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "dotd_${TAG}_checksums.txt"

# then confirm your archive matches the verified checksums file:
sha256sum -c --ignore-missing "dotd_${TAG}_checksums.txt"
```

### Build provenance

```sh
gh attestation verify dotd_${TAG}_linux_amd64.tar.gz --repo rocne/dot-dagger
```

(Requires the `gh` CLI; this repo's attestations are public.)
```

- [ ] **Step 2: Assert the section landed**

Run: `grep -n '## Verifying releases' README.md`
Expected: one match.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document cosign + SLSA release verification"
```

---

### Task 7: Open the PR

- [ ] **Step 1: Push and confirm no open PR exists yet**

Run:
```bash
gh pr list --head feature/claude-cosign-slsa --state open
git push -u origin feature/claude-cosign-slsa
```
Expected: empty PR list (none open yet), then a successful push.

- [ ] **Step 2: Create the PR**

```bash
gh pr create --title "ci: sign releases with cosign + SLSA provenance" --body "$(cat <<'EOF'
Implements the supply-chain hardening spec (`docs/superpowers/specs/2026-06-14-cosign-slsa-design.md`).

- GoReleaser keyless-signs the checksums file (cosign, Sigstore/Fulcio/Rekor).
- `actions/attest-build-provenance` emits SLSA build provenance over the archives.
- `id-token`/`attestations` permissions added to both release callers + the reusable callee.
- `e2e` job smoke-tests the published signature + provenance.
- `install.sh` verifies the signature when cosign is present (aborts only on a genuine bad signature; curl-only installs keep working).
- README "Verifying releases" section.

Homebrew is unaffected (archives unchanged → formula sha256 unchanged; signing fails closed before publish). Full live verification happens on the first release after merge — see the spec's testing section.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

### Task 8: First-release verification (manual, AFTER merge — do not skip)

The live signing path only runs in CI on a real release. After this PR merges and the next release-please release PR is merged (or a tag is cut), verify end-to-end. This is a checklist, not code.

- [ ] **Step 1: Confirm the new assets exist on the release**

Run: `gh release view <new-tag> --json assets -q '.assets[].name'`
Expected: includes `dotd_<tag>_checksums.txt.sig` and `dotd_<tag>_checksums.txt.pem` alongside the archives.

- [ ] **Step 2: Confirm the CI smoke test passed**

Check the release workflow run: the `e2e` job's "Verify release signature and provenance" step is green.

- [ ] **Step 3: Manually verify signature + provenance**

Run the README "Verifying releases" commands against the new tag. Both `cosign verify-blob` and `gh attestation verify` must succeed.

- [ ] **Step 4: Exercise install.sh both ways**

- With cosign installed: `curl -fsSL .../install.sh | sh` → prints `signature verified`.
- Without cosign (e.g. a container lacking it): same command → prints the `cosign not found` notice and still installs.

- [ ] **Step 5: Confirm Homebrew still works**

Run: `brew install rocne/tap/dot-dagger` (or `brew upgrade` if already tapped) → installs the new version cleanly.

- [ ] **Step 6: Update the TODO + close out**

Mark the 🔴 cosign+SLSA item done in `.claude/TODO.md` (local) and note completion in the next handoff.

---

## Self-review notes

- **Spec coverage:** signs block (Task 1) ✓; permissions across 3 files (Task 2) ✓; cosign-installer + provenance (Task 3) ✓; CI smoke test with its own cosign install (Task 4) ✓; install.sh enforcement with all three branches (Task 5) ✓; README docs incl. cosign v2+ (Task 6) ✓; first-release validation incl. Homebrew check (Task 8) ✓.
- **Identity regexp** is identical in install.sh (`${REPO}` → `rocne/dot-dagger`), the CI smoke test (`${{ github.repository }}`), and the README (literal) — all pin `_release.yml@` with any ref. Keep them in sync if the workflow filename ever changes.
- **No placeholders:** every code step shows the exact YAML/shell/markdown to add.
- **Pinned tool versions** (`goreleaser@v2.4.4`, `actionlint@v1.7.7`) for reproducible local checks; bump if a fetch 404s.
