# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors
- [ ] **Go public** — make repo public on GitHub (Settings → General → Change visibility)
- [ ] **Enable GitHub Pages** — after going public: Settings → Pages → Source: GitHub Actions. Docs workflow (`.github/workflows/docs.yml`) deploys automatically on merge to main.

## UX Review (from discoverability audit)

- [ ] **Missing `context` error needs guidance** — `predicate: env key "context" not set` should hint at `--env context=work` or `dotd env set context=work`. First-time users hit this immediately and have no path forward.
- [ ] **`apply` and `check` missing Long description** — help text is just the short summary line repeated. Should have prose + example invocations like `package generate` does.
- [ ] **`adopt` inference rules invisible** — help says "infers the destination" but doesn't say how. Should list the rules (executable → `bin/`, `.sh` → `shellrc/`, hidden file → `conf/dot-<name>`, etc.).
- [ ] **`link remove` scope unclear** — "remove owned symlinks" doesn't define "owned." Should clarify: symlinks whose target is inside the dotfiles repo.
- [ ] **`dotd files` bare could default to `list`** — single subcommand parent; bare invocation could just run it.
- [ ] **`--verbose` on `env show` does nothing visible** — could show source of each value (auto-detected / env.yaml / --env flag).

## Git / CI Infrastructure

- [ ] Multi-distro integration testing via Docker — spin up Ubuntu/Fedora containers to verify install + apply end-to-end (defer until repo is public)
