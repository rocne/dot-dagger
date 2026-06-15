# Design — nfpms deb/rpm packages with GPG signing

Date: 2026-06-15
Status: Approved, ready for implementation plan

## Goal

Ship `.deb` and `.rpm` packages as release artifacts so the binary installs on
Debian/Ubuntu (`apt install ./file`) and Fedora/RHEL (`dnf install ./file`)
without the Homebrew or `curl install.sh` paths. Packages are **GPG-signed** so
`rpm -K` verification passes today and a future hosted repo can enforce
`gpgcheck` without re-tooling.

This is the "DO NEXT" item from the distribution roadmap: cheapest meaningful
expansion of install surface, no hosting/accounts beyond a signing key.

## Scope

In scope:
- GoReleaser `nfpms` block producing `deb` + `rpm` for linux amd64 + arm64.
- GPG signing of both package formats.
- Public signing key distribution (committed + attached to release).
- CI wiring in `_release.yml` to make the key + passphrase available at build.
- Cleanup of the dead `windows` `format_override` in `archives`.

Out of scope (deferred, tracked elsewhere):
- Hosted package repo (gemfury/Cloudsmith) for by-name `apt/dnf install dotd`.
- Official Fedora/Debian repos.
- Install-in-CI verification of the packages — covered by the existing
  "Fedora e2e" TODO item, not this work.
- Man pages in the package payload — separate TODO; package ships the binary only.

## Design

### 1. nfpms block — `.goreleaser/dotd.yaml`

New top-level `nfpms:` section:

- `formats: [deb, rpm]`
- `builds: [dotd]` (reuses the existing linux/darwin × amd64/arm64 build; nfpms
  emits packages for the linux artifacts only)
- Metadata mirrored from the existing `brews` block:
  - maintainer: `Rocne Scribner <rocne.ks@gmail.com>`
  - homepage: `https://github.com/rocne/dot-dagger`
  - description: `Dotfiles manager — env resolution, DAG, symlinks, and packages`
  - `license: MIT`
- `bindir: /usr/bin` → binary installed as `/usr/bin/dotd`
- `contents:` the `dotd` binary only (no man pages yet)
- Signing:
  - `rpm.signature.key_file: "{{ .Env.GPG_KEY_PATH }}"`
  - `deb.signature.key_file: "{{ .Env.GPG_KEY_PATH }}"`
  - optional `rpm.signature.key_id` / `deb.signature.key_id` to pin the signing
    (sub)key if the key has multiple
  - passphrase supplied via `NFPM_DEFAULT_PASSPHRASE` env (read by GoReleaser/nfpm)

nfpm signs both formats **natively in Go** from the armored private `key_file` —
it does not shell out to `gpg`. There is therefore no keyring import, gpg-agent,
or pinentry to manage. The only CI requirement is: the armored private key on
disk at `GPG_KEY_PATH`, plus the passphrase env.

Note on signing value (documented so the trade-off is explicit):
- **rpm signing is verified** by `rpm -K` / dnf — the real win.
- **deb signing has ≈ zero practical value today.** nfpm's deb signing produces
  the `_gpgorigin` / debsigs format, which is verified only by `debsig-verify`
  with a configured policy that almost nobody installs — apt never checks it.
  Signed anyway for consistency and to be ready if a hosted apt repo lands later,
  but no current consumer verifies it.

### 2. GPG signing key (one-time, run locally by maintainer)

- Generate a dedicated signing key (RSA 4096 or ed25519), ASCII-armored export.
- Store as GitHub repo secrets:
  - `GPG_PRIVATE_KEY` — ASCII-armored private key
  - `GPG_PASSPHRASE` — key passphrase
- Public key:
  - committed to the repo at **`dotd-signing-key.asc` (repo root)** — a tracked,
    stable path. NOT under `dist/`, which is gitignored and wiped by GoReleaser
    `release --clean`.
  - attached to every GitHub release via GoReleaser `release.extra_files`,
    sourcing from that repo-root path.
- Users verify with `rpm --import dotd-signing-key.asc` (or the release asset)
  then `rpm -K dotd_<ver>_<arch>.rpm`.

### 3. CI wiring — `_release.yml`

Because nfpm signs natively (see §1), the wiring is minimal. Before the
GoReleaser step:
- Write `GPG_PRIVATE_KEY` (armored) to a temp file; export `GPG_KEY_PATH` → it.
- Export `NFPM_DEFAULT_PASSPHRASE` from `GPG_PASSPHRASE`.

No `gpg --import`, no gpg-agent, no pinentry. GoReleaser's nfpms step reads the
key file + passphrase env and signs the packages.

The signing secrets must reach whatever job runs GoReleaser — confirm they are
passed through the reusable-workflow boundary (`_release.yml` is called by both
release-please and the manual-tag path; both must surface the secrets). Missing
secrets fail loud, not silent: `key_file: {{ .Env.GPG_KEY_PATH }}` errors the
GoReleaser template if the env is unset, aborting the build rather than emitting
unsigned packages.

Effort: this section is the bulk of the work but is small — secret plumbing plus
one "write key to file" step, not gpg toolchain wrangling.

### 4. Cleanup (same file)

Remove the dead `format_overrides` `windows`/`zip` entry from `archives` — there
is no `windows` in `goos`, so it never fires. (If Scoop/winget is ever wanted, a
windows build gets added deliberately at that point.)

## Verification

This PR (manual):
- Generate a **throwaway local GPG key** (not the real release key) and point
  `GPG_KEY_PATH` + `NFPM_DEFAULT_PASSPHRASE` at it for local builds. nfpm signing
  runs in `--snapshot` (it is part of the nfpms build, not the cosign `signs`
  block — which *is* skipped in snapshot), so signatures are produced locally.
- `goreleaser release --snapshot --clean` produces `.deb` + `.rpm` for amd64 +
  arm64 alongside the existing tar.gz/checksums.
- `rpm --import <throwaway-pubkey> && rpm -K dist/dotd_*_amd64.rpm` reports the
  signature.
- `go test ./... && go vet ./... && gofmt -l . && golangci-lint run` clean
  (golangci-lint now installed locally — run before push).

Release pipeline:
- First real release after merge produces signed `.deb`/`.rpm` + public key
  attached, alongside the existing signed checksums/tar.gz.

Install-in-CI verification is **not** part of this work — deferred to the
existing "Fedora e2e" TODO.

## Risks / open notes

- **Secret propagation** through the reusable `_release.yml` boundary must be
  explicit — both caller paths (release-please, manual-tag) must surface
  `GPG_PRIVATE_KEY` + `GPG_PASSPHRASE`. A missing secret fails loud (template
  error on `GPG_KEY_PATH`), so the failure mode is an aborted build, not silent
  unsigned packages — acceptable.
- **Passphrase env name precedence** — nfpm reads `NFPM_DEFAULT_PASSPHRASE` (and
  more specific `NFPM_<ID>_<PACKAGER>_PASSPHRASE`). Confirm `NFPM_DEFAULT_PASSPHRASE`
  is honored when GoReleaser drives nfpm; fall back to the per-packager vars if not.
- **deb signature is non-verifiable in practice** (see §1) — accept as cosmetic.
- nfpm signs natively, so there is **no gpg toolchain dependency** in the runner
  — removing the largest previously-assumed risk.

## Outputs

Each release gains, alongside existing tar.gz + checksums + cosign sigs:
- `dotd_<ver>_<arch>.deb` (amd64, arm64) — GPG-signed
- `dotd_<ver>_<arch>.rpm` (amd64, arm64) — GPG-signed
- `dotd-signing-key.asc` — public signing key
