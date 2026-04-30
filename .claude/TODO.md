# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## Features

- [ ] **Arbitrary composition for compose targets** — allow a `build:` key in `.dotd.yaml` to specify a command run instead of (or after) text concatenation. Output written to `~/.config/dot-dagger/generated/` same as today; synthetic node handed to linker unchanged. Enables compiled tools (Go, Rust, C) and other non-text transforms inside `bin/` compose targets. Natural pipeline slot: between `compose` and `links`.

## Git / CI Infrastructure

- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)
