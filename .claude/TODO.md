# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## UX / CLI

- [ ] **`dotd files` / `dotd list` command** — expose the active fileset as a subcommand. Candidates: `dotd files list`, `dotd files list --all`, `dotd files list --os linux --context work`. Iterate on exact shape. Also consider unifying "fileset" stage naming with whatever this command is called.
- [x] **Rename `scripts/` → `shellrc/`** — done. Convention names now configurable via `dotd.conventions` in root `.dotd.yaml`.

## Git / CI Infrastructure

- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)
