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
  - passphrase supplied via `NFPM_DEFAULT_PASSPHRASE` env (read by GoReleaser/nfpm)

Note on signing value (documented so the trade-off is explicit):
- **rpm signing is verified** by `rpm -K` / dnf — the real win.
- **deb per-file signing is largely cosmetic** today — apt verifies a repo's
  `Release` file, not standalone `.deb` GPG signatures. Signed anyway for
  consistency and to be ready if a hosted apt repo lands later.

### 2. GPG signing key (one-time, run locally by maintainer)

- Generate a dedicated signing key (RSA 4096 or ed25519), ASCII-armored export.
- Store as GitHub repo secrets:
  - `GPG_PRIVATE_KEY` — ASCII-armored private key
  - `GPG_PASSPHRASE` — key passphrase
- Public key:
  - committed to the repo at `dist/dotd-signing-key.asc`
  - attached to every GitHub release via GoReleaser `release.extra_files`
- Users verify with `rpm --import dist/dotd-signing-key.asc` (or the release asset)
  then `rpm -K dotd_<ver>_<arch>.rpm`.

### 3. CI wiring — `_release.yml`

Before the GoReleaser step:
- Import `GPG_PRIVATE_KEY` into the runner keyring (headless: loopback pinentry).
- Write the private key to a temp file path; export `GPG_KEY_PATH` pointing at it.
- Export `NFPM_DEFAULT_PASSPHRASE` from `GPG_PASSPHRASE`.

GoReleaser's nfpms step reads `GPG_KEY_PATH` + `NFPM_DEFAULT_PASSPHRASE` at build
time and signs the packages. The headless gpg-agent / passphrase handling is the
fiddly part of this work; budget debugging time here.

The signing secrets must be available to whatever job runs GoReleaser — confirm
they are passed through the reusable-workflow boundary (`_release.yml` is called
by both release-please and the manual-tag path; both must surface the secrets).

### 4. Cleanup (same file)

Remove the dead `format_overrides` `windows`/`zip` entry from `archives` — there
is no `windows` in `goos`, so it never fires. (If Scoop/winget is ever wanted, a
windows build gets added deliberately at that point.)

## Verification

This PR (manual):
- `goreleaser release --snapshot --clean` locally produces `.deb` + `.rpm` for
  amd64 + arm64 alongside the existing tar.gz/checksums.
- With a local test key, `rpm -K dist/dotd_*_amd64.rpm` reports the signature.
- `go test ./... && go vet ./... && gofmt -l . && golangci-lint run` clean
  (golangci-lint now installed locally — run before push).

Release pipeline:
- First real release after merge produces signed `.deb`/`.rpm` + public key
  attached, alongside the existing signed checksums/tar.gz.

Install-in-CI verification is **not** part of this work — deferred to the
existing "Fedora e2e" TODO.

## Risks / open notes

- Headless GPG in CI is the main failure mode (passphrase prompts, gpg-agent
  state). Mitigate with loopback pinentry and a clean key import step.
- nfpms rpm signing requires the rpm signing toolchain present in the runner;
  GoReleaser's container generally has it, but verify on first run.
- Secret propagation through the reusable `_release.yml` boundary must be
  explicit — a missing secret silently produces unsigned packages rather than
  failing loudly; the snapshot/verify step should assert the signature exists.

## Outputs

Each release gains, alongside existing tar.gz + checksums + cosign sigs:
- `dotd_<ver>_<arch>.deb` (amd64, arm64) — GPG-signed
- `dotd_<ver>_<arch>.rpm` (amd64, arm64) — GPG-signed
- `dotd-signing-key.asc` — public signing key
