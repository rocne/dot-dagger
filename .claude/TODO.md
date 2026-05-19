# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## Features

- [x] **Unified action system** — implemented. `@action <type>`, `actions:` key in `.dagger`, aliases (`@source`/`@no-source`/`@symlink`/`compose: true`), sequencing validation all done. Convention dirs use explicit `.dagger` defaults by design — not implicit magic.
- [x] **`@disable` annotation** — implemented. Walk skips disabled files; disabled paths logged at debug level.
- [x] **BasicNode completeness** — `after`, `require`, `request`, `disable` all expressible in `.dagger` `files:` dict. `internal/manifest` and §20 dropped.
- [x] **`compose: true` alias** — works as shorthand for `composition.enabled: true` in `.dagger`.
- [ ] **TTY-aware missing-key prompt** (M3) — currently always halts with hint; no interactive fallback. Deferred.
- [ ] **`dotd init` rc-file check** (M8) — `internal/setup` has `AppendSourceLine`/`HasSourceLine` but never called from `init_cmd.go`. Deferred.

## Git / CI Infrastructure

- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)
