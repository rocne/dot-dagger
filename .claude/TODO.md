# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## Features

- [ ] **Unified action system** — specced in `actions.md`. Implement `@action <type>` annotation and `actions:` key in `.dotd.yaml`; wire `@source`/`@no-source`/`@symlink`/`compose: true` as aliases; implement convention dir defaults as implicit actions; enforce sequencing rules and error cases.

## Git / CI Infrastructure

- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)
