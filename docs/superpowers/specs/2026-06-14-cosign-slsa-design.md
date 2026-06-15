# Design — Release supply-chain hardening: cosign signing + SLSA provenance

**Date:** 2026-06-14
**Status:** Approved (design phase); next step writing-plans.
**Scope:** Sign release artifacts with keyless cosign and emit SLSA build
provenance, document and enforce verification, with zero disruption to the
existing GoReleaser / release-please / Homebrew flow.

## Goal

Today a release publishes unsigned `.tar.gz` archives plus a
`dotd_<tag>_checksums.txt`. A checksums file alone proves nothing about origin —
an attacker who swaps an archive swaps its checksum too. Nothing cryptographically
ties the artifacts to *this* repo's build.

Add the current best-practice, no-private-keys supply-chain stack:

1. **Cosign signatures** (keyless, Sigstore/Fulcio/Rekor) over the checksums file
   — proves the release was produced by the `dot-dagger` release workflow.
2. **SLSA build provenance** (GitHub `actions/attest-build-provenance`) over the
   archives — a signed statement of how/where each artifact was built.
3. **Verification** — documented for all consumers, and enforced best-effort in
   `install.sh`.

Both run entirely in CI via GitHub Actions OIDC. No keys or secrets to store.

## Non-goals (explicit scope boundaries)

- No changes to the `dotd` binary or its behavior.
- No per-archive signing — signing the checksums file once covers all artifacts
  (the checksums file already hashes every archive).
- No Homebrew formula signature check — Homebrew has its own sha256 integrity
  model; a `cosign verify` in the formula is non-standard and low-value. Deferred.
- No interactive confirmation in `install.sh` — a `curl … | sh` pipe has no TTY.

## Homebrew impact: none

Signing does **not** touch the archives, so their sha256 is unchanged. GoReleaser's
`brews:` block generates the formula from those same archive sha256s, so the
formula is identical to today and points at the same archive URLs. The new
`.sig`/`.pem`/attestation files are separate release assets the formula never
references, so `brew install` never downloads them. Homebrew validates via its own
formula sha256 (as always), not via our signatures. The signing pipeline runs
**before** publish (`build → archive → checksum → sign → publish`, and `brews:` is
part of publish), so a signing failure aborts the run before anything is
published — no half-released or corrupted Homebrew state. Fails closed.

## Components

### 1. GoReleaser — `.goreleaser/dotd.yaml`

Add a `signs:` block performing keyless cosign blob-signing of the checksums
artifact. GoReleaser substitutes `${artifact}` (the checksums file),
`${signature}` (defaults to `${artifact}.sig`), and `${certificate}`.

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

Produces, alongside the existing assets:
- `dotd_<tag>_checksums.txt.sig`
- `dotd_<tag>_checksums.txt.pem`

`--yes` suppresses the interactive confirmation; keyless signing uses the ambient
GitHub OIDC token (no `COSIGN_EXPERIMENTAL` needed with modern cosign). `signature:`
and `certificate:` are set explicitly rather than relying on GoReleaser defaults.

**Public transparency log:** keyless signing records the artifact hash and the
signing identity (the workflow's OIDC SAN) in the public Rekor log. No secrets are
exposed, and the repo is already public, but the release identity becomes publicly
queryable by design — this is how `cosign verify-blob` proves origin.

### 2. Workflow — `_release.yml` (callee) + both callers

**`_release.yml`:**
- Add a `sigstore/cosign-installer@v3` step in the `release` job, before the
  GoReleaser step, so the `cosign` binary is on PATH.
- After the GoReleaser step, add an `actions/attest-build-provenance@v2` step with
  `subject-path: dist/dotd_*.tar.gz` (the built archives) to emit SLSA provenance
  into GitHub's attestation store.
- Add `id-token: write` and `attestations: write` to the workflow permissions.
- **Post-release CI smoke test:** in the `e2e` job (which already downloads the
  published release), add a `cosign verify-blob` against the published checksums
  file using the same identity regexp + issuer as `install.sh`, and a
  `gh attestation verify` against one archive. This catches a broken `signs:`
  config or wrong identity *in CI* rather than on a user's first install. Near-zero
  cost; the `e2e` job already exists and runs after `release`.
  **Note:** the `e2e` job is a *separate job/runner* from `release`, so it needs its
  own `sigstore/cosign-installer@v3` step (the `release` job's cosign is not on this
  runner). `gh` is preinstalled on GitHub runners, and the job already exports
  `GH_TOKEN`, so `gh attestation verify` needs no extra setup.

**Permission propagation (the cross-cutting detail):** a reusable workflow cannot
grant itself more permission than its caller. Both caller `release` jobs currently
declare only `contents: write` + `issues: write`. Add `id-token: write` and
`attestations: write` to the `release` job in **all three** files:
- `.github/workflows/release-please.yml` (primary caller)
- `.github/workflows/release.yml` (break-glass caller)
- `.github/workflows/_release.yml` (callee)

`id-token: write` is required for keyless cosign OIDC; `attestations: write` is
required for `attest-build-provenance` to write to the attestation store.

**Signing identity (drives the verify regexp).** Signing always runs inside
`_release.yml`, so the Fulcio cert's SAN is built from `job_workflow_ref` —
`https://github.com/rocne/dot-dagger/.github/workflows/_release.yml@<ref>`. The
`<ref>` differs by caller: `refs/heads/main` when release-please triggers the
release, `refs/tags/v<x>` when a manual tag does. Therefore the verify identity
**cannot pin the ref**; it pins the workflow file and allows any ref:
`^https://github\.com/rocne/dot-dagger/\.github/workflows/_release\.yml@`. This is
tighter than a repo-wide match (rejects signatures from any other workflow) while
surviving both release paths. The same regexp is used in `install.sh` and the
README verify command — keep all three in sync.

### 3. `install.sh` — enforced verification

Insert a cosign-verification step **after** the existing checksum verification and
**before** extract/install. Behavior:

1. Attempt to download `dotd_<tag>_checksums.txt.sig` and `.pem` (best-effort, do
   not `-f`-fail the whole script on 404).
2. Branch:
   - **`cosign` on PATH and both sig+cert downloaded** → run:
     ```sh
     cosign verify-blob \
       --certificate "$TMP/$CHECKSUMS.pem" \
       --signature   "$TMP/$CHECKSUMS.sig" \
       --certificate-identity-regexp "^https://github\.com/rocne/dot-dagger/\.github/workflows/_release\.yml@" \
       --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
       "$TMP/$CHECKSUMS"
     ```
     - verifies → print `signature verified`, proceed.
     - **fails (non-zero) → print error and `exit 1` (hard abort, no install).**
       A present-but-invalid signature is a tampering signal.
     - Requires **cosign v2+** (the `--certificate-identity*` flags are v2 syntax).
   - **`cosign` present but sig/cert missing** (e.g. a pre-signing release) → print
     a one-line notice, fall back to checksum-only, proceed.
   - **`cosign` absent** → print a one-line notice
     (`cosign not found — skipping signature verification; install cosign for full verification`),
     fall back to checksum-only, proceed. **curl-only installs keep working.**

UX summary: automatic everywhere; one extra `signature verified` line on success;
warn-and-continue when verification is *not possible*; abort **only** on a genuine
bad signature.

### 4. README — "Verifying releases"

Add a section with copy-paste commands:
- **Signature** (requires **cosign v2+**): download the archive, the checksums
  file, and its `.sig`+`.pem`; run `cosign verify-blob` with the same
  `--certificate-identity-regexp`
  (`^https://github\.com/rocne/dot-dagger/\.github/workflows/_release\.yml@`) and
  `--certificate-oidc-issuer` as install.sh, then `sha256sum -c` the archive
  against the verified checksums file.
- **Provenance:** `gh attestation verify dotd_<tag>_<os>_<arch>.tar.gz --repo rocne/dot-dagger`
  (a public repo's attestations are readable; `gh` must be installed).

## Error handling

- **CI signing failure** → release run fails before publish; nothing released;
  re-run. Fails closed (see Homebrew section).
- **CI attestation failure** → the attestation step runs after GoReleaser has
  published archives + formula, so the release is published but momentarily
  un-attested. This is **recoverable, not a corruption**: re-run
  `attest-build-provenance` against the already-published archives — provenance is
  not bound to the publish moment. The only cost is that a re-run records a
  different workflow run-id as the builder. The signed checksums already protect
  integrity, so provenance is purely additive. The job still goes red on failure,
  prompting the re-run.
- **install.sh:** bad signature → abort; can't-verify → warn + continue;
  no-cosign → warn + continue.

## Testing / validation

- **Local dry-run:** `goreleaser release --snapshot --clean --skip=sign,publish`
  confirms the artifact layout and that the config parses. True keyless signing
  cannot run locally (needs CI's OIDC token), so signing itself is CI-verified.
- **First real release after merge:**
  1. Confirm `.sig`, `.pem`, and the provenance attestation exist for the release.
  2. Run `cosign verify-blob …` and `gh attestation verify …` by hand — both pass.
  3. Run `install.sh` end-to-end on a machine with cosign (expect
     `signature verified`) and without (expect the notice + checksum-only).
  4. Confirm `./test/run-e2e-release.sh` is still green (it selects assets by exact
     name like install.sh, so extra artifacts are inert).
- **Tamper check (optional):** corrupt a downloaded archive/checksums locally and
  confirm install.sh aborts.

## Files touched

| File | Change |
|------|--------|
| `.goreleaser/dotd.yaml` | add `signs:` block |
| `.github/workflows/_release.yml` | `release` job: cosign-installer + attest-build-provenance steps, `id-token`+`attestations` perms. `e2e` job: cosign-installer + post-release `cosign verify-blob` / `gh attestation verify` smoke test |
| `.github/workflows/release-please.yml` | add `id-token`+`attestations` to `release` job |
| `.github/workflows/release.yml` | add `id-token`+`attestations` to `release` job |
| `install.sh` | cosign verify-blob step after checksum verification |
| `README.md` | "Verifying releases" section |

## Open questions

None — design approved. Validation of the live signing path is inherently
deferred to the first real release (CI-only OIDC).
