# E2E Test Coverage Design

**Date:** 2026-06-04
**Status:** Approved

## Goal

Achieve complete e2e test coverage for all `dotd` commands. Current gaps:

- `unapply` — no integration or shell e2e coverage (unit tests only)
- `compose` — no shell e2e coverage (integration_test.go only)
- `setup` / `init` / `teardown` — no integration or shell e2e coverage
- `config` / `env` subcommands — no integration or shell e2e coverage
- `adopt`, `bundle` — no integration or shell e2e coverage
- `dag check` — no shell e2e coverage (integration_test.go only)
- `package check` / `package generate` — no shell e2e coverage (integration_test.go only)

## Test Layers

Each gap gets coverage at **both** layers:

- **Shell e2e** — real binary in Docker, assertions in sh/expect
- **Go integration tests** — in-process via `//go:build integration`, in `integration_test.go`

Exception: where existing integration tests already give thorough coverage (compose, package generate), shell e2e is added but no new Go integration tests are written.

## Infrastructure Changes

### Docker (both `Dockerfile` and `Dockerfile.local`)

Add `expect` to enable interactive TTY testing:

```dockerfile
RUN apt-get install -y --no-install-recommends expect
```

### `test/run-e2e.sh`

Add a second runner function alongside `run_test`:

```sh
run_expect_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    dotd-e2e \
    sh -c ". /procure/local.sh && expect /tests/${EXERCISER}"
}
```

Expect scripts live in `test/e2e/` alongside shell scripts, with `.exp` extension.

### E2E Fixture (`test/e2e/fixture/`)

Extend for compose (Batch 2). Add:

```
shellrc/
  dot-extras.sh.d/
    .dagger          # compose target declaration
    base.sh          # active fragment (EXTRAS_BASE marker)
    nosync-work.sh   # context=work-gated fragment (EXTRAS_WORK marker)
```

Update `shellrc/.dagger` to declare `dot-extras.sh.d` as a compose target that generates `extras.sh`, sourced from init.sh.

No other fixture changes needed. Config/env/setup tests write their own temp config files via `--config <path>`.

## Batches

### Batch 1 — `unapply`

**Shell e2e:**

`test/e2e/unapply.sh`
- apply (linux/personal)
- `unapply --yes`
- assert symlinks removed, init.sh removed

`test/e2e/unapply-cancel.exp`
- apply (linux/personal)
- spawn `unapply`, expect confirm prompt, send "n"
- assert symlinks still present, exit 0

**Integration tests** (new in `integration_test.go`):

`TestUnapplyAfterApply`
- apply, then unapply with `--yes` flag
- assert symlinks gone, init.sh absent

`TestUnapplyCancel`
- apply, then unapply piping "n" to stdin
- assert symlinks preserved

Each script is self-contained: `unapply-cancel.exp` runs apply via `exec sh -c` before spawning `unapply`.

**Runner additions:**
```sh
run_test unapply.sh
run_expect_test unapply-cancel.exp
```

---

### Batch 2 — `compose`

**Fixture extension** (described in Infrastructure section above).

**Shell e2e:**

`test/e2e/compose.sh`
- apply (linux/personal) — assert generated `extras.sh` exists and sourced in init.sh
- `compose list` — assert `extras.sh` appears
- `compose check` — assert exit 0 after clean apply
- overwrite generated file with stale content, `compose check` — assert "stale" in output

**Integration tests:** already complete; none added.

**Runner additions:**
```sh
run_test compose.sh
```

---

### Batch 3 — `setup` / `init` / `teardown`

**Shell e2e:**

`test/e2e/setup.exp`
- spawn `dotd setup --config /tmp/dotd.yaml`
- answer prompts: files → `/fixture`, env-file → `/fixture/env.yaml`, link-root → `/home/e2e`, bin-dir → `/home/e2e/bin`, init-file → `/tmp/init.sh`, generated-dir → `/tmp/generated`
- assert `/tmp/dotd.yaml` exists with expected keys

`test/e2e/teardown.exp` (two scenarios in one script)
- scenario A: spawn `dotd teardown --config /tmp/dotd.yaml`, send "y", assert config removed
- scenario B: re-run setup, spawn teardown, send "n", assert config preserved, exit 0

`test/e2e/init.sh`
- pre-create a minimal config file at `/tmp/dotd.yaml` pointing at a writable temp dotfiles dir
- run `dotd init --config /tmp/dotd.yaml`
- assert `.dagger` files scaffolded in the temp dotfiles dir

**Integration tests** (new in `integration_test.go`):

`TestSetupThenTeardown`
- run setup via `ienv`, verify config written
- run teardown with `--yes`, verify config removed

`TestInitAfterSetup`
- run setup, then init
- verify `.dagger` scaffolded in dotfiles dir

**Runner additions:**
```sh
run_expect_test setup.exp
run_expect_test teardown.exp
run_test init.sh
```

---

### Batch 4 — `config` / `env`

**Shell e2e:**

`test/e2e/config-cmds.sh`
- write a minimal config file to `/tmp/dotd.yaml`
- `config show --config /tmp/dotd.yaml` — assert output contains known key
- `config get --config /tmp/dotd.yaml <key>` — assert value
- `config set --config /tmp/dotd.yaml <key> <new-value>` — then `config get` to verify

`test/e2e/env-cmds.sh`
- copy `/fixture/env.yaml` to `/tmp/env.yaml` (fixture is read-only; `env set` must write)
- `env show --env-file /tmp/env.yaml` — assert known keys present
- `env get --env-file /tmp/env.yaml <key>` — assert value
- `env set --env-file /tmp/env.yaml <key> <value>` — then `env get` to verify
- `env diff --env-file /tmp/env.yaml --env os=linux` — assert override appears in output

**Integration tests** (new in `integration_test.go`):

`TestConfigCmdsLifecycle`
- config show on empty config → no error
- config set key → config get returns value

`TestEnvCmdsLifecycle`
- env show with fixture env.yaml → known keys present
- env set → env get returns new value
- env diff with --env override → shows override

**Runner additions:**
```sh
run_test config-cmds.sh
run_test env-cmds.sh
```

---

### Batch 5 — `adopt`, `bundle`, `dag check`, `package` subcommands

**Shell e2e:**

`test/e2e/adopt.sh`
- copy fixture to writable temp dir
- create a new shell script in temp dir
- `adopt <script> --files <temp-dir> --to shellrc`
- assert file moved into `shellrc/`, assert `.dagger` updated (or created)

`test/e2e/bundle.sh`
- `bundle <script>` against fixture (linux/personal)
- assert output is a self-contained script containing source lines for all transitive deps

`test/e2e/dag-check.sh`
- `dotd dag check` against fixture (linux/personal) — assert exit 0
- assert output lists scripts in dependency order

`test/e2e/package-check.sh`
- `package check` (linux/personal) — assert fake-installed shows as installed
- `package generate` (linux/personal) — assert exit 0
- add a script with `@require not-installable`, `package generate` — assert exit non-zero

**Integration tests** (new in `integration_test.go`):

`TestAdoptShellScript_Integration`
- adopt a new shell script into testdata fixture copy
- verify file at new path, .dagger updated

`TestBundleOutput_Integration`
- bundle `aliases.sh` (depends on base)
- verify output contains base.sh content inlined

Skip `package generate` hard-fail and `dag check` — already covered in `integration_test.go`.

**Runner additions:**
```sh
run_test adopt.sh
run_test bundle.sh
run_test dag-check.sh
run_test package-check.sh
```

## Delivery Order

| Batch | New shell e2e scripts | New integration tests |
|-------|----------------------|-----------------------|
| 1 | `unapply.sh`, `unapply-cancel.exp` | `TestUnapplyAfterApply`, `TestUnapplyCancel` |
| 2 | `compose.sh` + fixture extension | none |
| 3 | `setup.exp`, `teardown.exp`, `init.sh` | `TestSetupThenTeardown`, `TestInitAfterSetup` |
| 4 | `config-cmds.sh`, `env-cmds.sh` | `TestConfigCmdsLifecycle`, `TestEnvCmdsLifecycle` |
| 5 | `adopt.sh`, `bundle.sh`, `dag-check.sh`, `package-check.sh` | `TestAdoptShellScript_Integration`, `TestBundleOutput_Integration` |

Each batch is independently mergeable. Batches 1–2 are highest priority (largest coverage gap, highest user impact).
