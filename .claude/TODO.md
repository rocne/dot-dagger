# TODO / Deferred Tasks

Items that are known but intentionally deferred. Update this as things get done or new items come up.

---

## Project Setup

- [ ] Add `LICENSE` (MIT decided, just needs the file)
- [ ] Initialize `go.mod` (defer until first `.go` file is written)

---

## Documentation

- [ ] `CONTRIBUTING.md` — defer until project goes public or has external contributors

## Git / CI Infrastructure

- [ ] GitHub Actions CI workflow (build + test) — defer until `go.mod` exists
- [ ] GitHub Actions lint workflow (`golangci-lint`) — defer until `go.mod` exists
- [ ] PR template (`.github/pull_request_template.md`)

## Implementation

_(To be filled in as planning begins)_

---

## Architecture / Design

- [ ] Consider splitting dot-dagger into separate tools: a standalone DAG/source-order resolver, and a separate linker tool that handles symlinking. Source order resolution feels potentially independent from the linking concern. Revisit before implementation begins.
- [ ] Review `link_root` and `@symlink` relative path semantics more carefully before finalising spec. Current resolution: `@symlink` destinations are implicit-relative to `link_root` if they don't start with `/` or `~/`. Needs validation against real use cases.
