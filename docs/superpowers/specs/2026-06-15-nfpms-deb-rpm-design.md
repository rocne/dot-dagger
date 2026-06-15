# Design — nfpms deb/rpm packages with GPG signing

Date: 2026-06-15
Status: Approved, ready for implementation plan

## Goal

Ship `.deb` and `.rpm` packages as release artifacts so the binary installs on
Debian/Ubuntu (`apt install ./file`) and Fedora/RHEL (`dnf install ./file`)
without the Homebrew or `curl install.sh` paths. Packages are **GPG-signed** so
`rpm -K` verification passes today and a future hosted **yum** repo can enforce
`gpgcheck` reusing the same per-package rpm signature without re-tooling. (A
hosted apt repo signs its `Release` metadata with the repo key separately — the
deb per-package signature does not carry forward; see §1 on deb signing value.)

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
- **No explicit `contents:` for the binary** — GoReleaser nfpms auto-includes the
  `builds:` binaries at `bindir`. Hand-listing it risks double-packaging.
  `contents:` is only for extra files, of which there are none here.
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

Packages are **also** covered by the existing cosign setup for free: GoReleaser
lists `.deb`/`.rpm` in `checksums.txt`, and the current `signs: artifacts:
checksum` block cosigns that file. So each package gets its GPG signature *plus*
transitive cosign coverage via the signed checksums — do **not** add a separate
cosign blob-signing pass for the packages.

Note on signing value (documented so the trade-off is explicit):
- **rpm signing is verified** by `rpm -K` / dnf — the real win.
- **deb signing has ≈ zero practical value today.** nfpm's deb signing produces
  the `_gpgorigin` / debsigs format, which is verified only by `debsig-verify`
  with a configured policy that almost nobody installs — apt never checks it.
  Signed anyway for consistency and to be ready if a hosted apt repo lands later,
  but no current consumer verifies it.

### 2. GPG signing key (one-time, run locally by maintainer)

- Generate a dedicated signing key, **RSA 4096** (not EdDSA — older rpm/dnf on
  RHEL/EPEL can fail to verify EdDSA rpm signatures), **non-expiring** (an expiry
  silently breaks `rpm -K` on already-shipped packages after the date; if a
  finite expiry is used, document a rotate-and-republish plan). ASCII-armored export.
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

**Secret flow needs no caller changes (verified).** Both callers
(`release.yml`, `release-please.yml`) invoke `_release.yml` with
`secrets: inherit`, so the new `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` repo secrets
reach the reusable workflow automatically. The only edits to `_release.yml`:
a "write key to temp file" shell step, and two `env:` entries
(`GPG_KEY_PATH`, `NFPM_DEFAULT_PASSPHRASE`) on the existing GoReleaser step.

Missing secrets fail loud, not silent: `key_file: {{ .Env.GPG_KEY_PATH }}`
errors the GoReleaser template if the env is unset, aborting the build rather
than emitting unsigned packages.

Effort: small — one shell step plus two env lines, no gpg toolchain wrangling.

### 4. Cleanup (same file)

Remove the dead `format_overrides` `windows`/`zip` entry from `archives` — there
is no `windows` in `goos`, so it never fires. (If Scoop/winget is ever wanted, a
windows build gets added deliberately at that point.)

## Verification

This PR (manual):
- **Validate first:** confirm nfpm signing actually runs under `--snapshot`
  (expected, since it is inline in the nfpms build rather than the cosign `signs`
  block — which *is* snapshot-skipped). If GoReleaser skips it in snapshot, fall
  back to a full non-snapshot dry build or standalone `nfpm pkg` to verify the
  signature path before wiring CI.
- Generate a **throwaway local GPG key** (not the real release key) and point
  `GPG_KEY_PATH` + `NFPM_DEFAULT_PASSPHRASE` at it for local builds.
- `goreleaser release --snapshot --clean` produces `.deb` + `.rpm` for amd64 +
  arm64 alongside the existing tar.gz/checksums.
- `rpm --import <throwaway-pubkey> && rpm -K dist/*.rpm` reports the signature
  (rpm names use `x86_64`/`aarch64`, not `amd64`/`arm64` — glob `*.rpm`).
- `go test ./... && go vet ./... && gofmt -l . && golangci-lint run` clean
  (golangci-lint now installed locally — run before push).

Release pipeline:
- First real release after merge produces signed `.deb`/`.rpm` + public key
  attached, alongside the existing signed checksums/tar.gz.

Install-in-CI verification is **not** part of this work — deferred to the
existing "Fedora e2e" TODO.

## Risks / open notes

- **Secret propagation is resolved, not a risk** (see §3): both callers use
  `secrets: inherit`, so `GPG_PRIVATE_KEY` + `GPG_PASSPHRASE` reach `_release.yml`
  with no caller edits. A missing secret still fails loud (template error on
  `GPG_KEY_PATH`) → aborted build, never silent unsigned packages.
- **Passphrase env name precedence** (the one real open item) — nfpm reads
  `NFPM_DEFAULT_PASSPHRASE` and the more specific `NFPM_<ID>_<PACKAGER>_PASSPHRASE`.
  Confirm `NFPM_DEFAULT_PASSPHRASE` is honored when GoReleaser drives nfpm; fall
  back to the per-packager vars if not.
- **deb signature is non-verifiable in practice** (see §1) — accept as cosmetic.
- nfpm signs natively, so there is **no gpg toolchain dependency** in the runner.

### Intentional omissions (not TODO — deliberate YAGNI)
- deb `Section`/`Priority`, rpm `Group` — left to nfpm defaults; their absence
  only yields cosmetic `lintian`/`rpmlint` warnings, never install failures.
- Package version is **not** set manually — GoReleaser feeds the release tag to
  nfpms automatically, matching the binary's stamped version.

## First-release preconditions (runbook)

The first release after this merge breaks unless all three are in place *before*
it fires (order matters; each missing piece fails the build differently):

1. **Key generated** (RSA 4096, non-expiring) locally by the maintainer.
2. **Secrets set** — `GPG_PRIVATE_KEY` + `GPG_PASSPHRASE` added to repo secrets.
   Missing → GoReleaser template error on `GPG_KEY_PATH`.
3. **Public key committed** at repo-root `dotd-signing-key.asc`. Missing →
   `release.extra_files` fails (referenced file absent).

Recommended sequence: generate key → set secrets → commit config + public key in
this PR → merge → release. The PR itself contains the public key and config, so
1–3 are satisfied by the time the next release PR merges.

## Outputs

Each release gains, alongside existing tar.gz + checksums + cosign sigs:
- `.deb` × 2 — GPG-signed. Conventional names use deb arch: `..._amd64.deb`,
  `..._arm64.deb`.
- `.rpm` × 2 — GPG-signed. Conventional names use **rpm** arch: `...x86_64.rpm`,
  `...aarch64.rpm` (note: not `amd64`/`arm64` — they differ from the deb names).
- `dotd-signing-key.asc` — public signing key.

Filenames use nfpms' default conventional templates (deb and rpm each follow
their ecosystem's convention) — intentionally *not* the custom archive
`name_template`, since package tooling expects the conventional forms.
