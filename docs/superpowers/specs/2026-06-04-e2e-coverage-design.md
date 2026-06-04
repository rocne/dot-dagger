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
- macOS predicate path — integration_test.go only, no shell e2e

Out of scope: `config edit` and `env edit` (both launch `$EDITOR`, untestable in CI).

## Test Layers

Each gap gets coverage at **both** layers:

- **Shell e2e** — real binary in Docker, assertions in sh or expect
- **Go integration tests** — in-process via `//go:build integration`, in `integration_test.go`

Exception: where existing integration tests already give thorough coverage (compose, dag check, package generate), shell e2e is added but no new Go integration tests are written.

## Infrastructure Changes

### Docker (`Dockerfile` and `Dockerfile.local`)

Add `expect`:

```dockerfile
RUN apt-get install -y --no-install-recommends expect
```

Add explicit `COPY` lines for each new `.exp` file alongside existing `.sh` copies. Both Dockerfiles use per-file COPY lines (no glob support in this image), so each new script — `.sh` or `.exp` — needs its own line.

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

Expect scripts live in `test/e2e/` with `.exp` extension. Each is self-contained: interactive expect scripts run any required setup steps (e.g., apply) inline via `exec` before spawning the command under test.

### `integration_test.go` — harness extension

Add `runWithStdin` to `ienv` to support piped stdin for interactive integration tests:

```go
func (e *ienv) runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
    t.Helper()
    base := []string{ /* same flags as runMayFail */ }
    var buf bytes.Buffer
    cmd := newRootCmd()
    cmd.SetIn(stdin)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    cmd.SetArgs(append(base, args...))
    return buf.String(), cmd.Execute()
}
```

Used for `TestUnapplyCancel`. Setup/teardown integration tests call `newRootCmd()` directly (same pattern as existing unit tests) since they manage their own XDG paths, not `ienv` paths.

### E2E Fixture (`test/e2e/fixture/`)

Extend for compose (Batch 2). Add:

```
shellrc/
  dot-extras.sh.d/
    .dagger          # compose target declaration
    base.sh          # always-active fragment; contains "EXTRAS_BASE" marker string
    nosync-work.sh   # context=work-gated fragment; contains "EXTRAS_WORK" marker string
```

Update `shellrc/.dagger` to declare `dot-extras.sh.d` as a compose target generating `extras.sh`, sourced from init.sh.

No other fixture changes. Tests that need a writable dotfiles dir copy the fixture to a temp dir at runtime.

## Batches

---

### Batch 1 — `unapply`

Prerequisite: implement `ienv.runWithStdin` in `integration_test.go` (described above).

**Shell e2e:**

`test/e2e/unapply.sh`
- apply (linux/personal)
- `unapply --yes`
- assert: `.zshrc` symlink absent, `init.sh` absent, `bin/hello` symlink absent

`test/e2e/unapply-cancel.exp`
- `exec sh -c "dotd apply ..."` to set up state
- spawn `dotd unapply ...` (no `--yes`)
- expect confirm prompt, send `"n\r"`
- assert: exit 0, `.zshrc` symlink still present

**Integration tests** (new in `integration_test.go`):

`TestUnapplyAfterApply`
- apply (linux/personal), then `unapply --yes`
- assert: symlinks gone, `initFile` absent

`TestUnapplyCancel`
- apply (linux/personal)
- call `runWithStdin(t, strings.NewReader("n\n"), "unapply")`
- assert: exit 0, symlinks preserved

**Dockerfiles:** add `COPY unapply.sh /tests/unapply.sh` and `COPY unapply-cancel.exp /tests/unapply-cancel.exp`.

**Runner additions:**
```sh
run_test unapply.sh
run_expect_test unapply-cancel.exp
```

---

### Batch 2 — `compose` + macOS predicate

**Fixture extension** (described in Infrastructure section above).

**Shell e2e:**

`test/e2e/compose.sh`
- apply (linux/personal)
- assert: `$GENERATED_DIR/extras.sh` exists and is sourced in `init.sh`
- `compose list` — assert `extras.sh` appears in output
- `compose check` — assert exit 0 (clean state)
- overwrite `$GENERATED_DIR/extras.sh` with stale content
- `compose check` — assert output contains "stale"

`test/e2e/macos-apply.sh`
- apply `--env os=macos --env context=personal` (Linux binary, macOS predicate via flag)
- assert: `macos.sh` in `init.sh`, `linux.sh` absent

**Integration tests:** compose already complete; macOS already covered by `TestApplyMacOSPersonal`. None added.

**Dockerfiles:** add `COPY compose.sh`, `COPY macos-apply.sh`.

**Runner additions:**
```sh
run_test compose.sh
run_test macos-apply.sh
```

---

### Batch 3 — `setup` / `init` / `teardown`

**Shell e2e:**

`test/e2e/setup.exp`
- spawn `dotd setup --config /tmp/dotd.yaml`
- for each prompt: send the fixture path or accept default with `"\r"`
- assert: `/tmp/dotd.yaml` exists and contains `dotfiles:` key

`test/e2e/teardown-confirm.exp`
- write `/tmp/dotd.yaml` with `dotfiles: /fixture\n`
- spawn `dotd teardown --config /tmp/dotd.yaml --files /fixture --env-file /fixture/env.yaml`
- expect confirm prompt, send `"y\r"`
- assert: `/tmp/dotd.yaml` absent after exit

`test/e2e/teardown-cancel.exp`
- same setup, spawn teardown, send `"n\r"`
- assert: exit 0, `/tmp/dotd.yaml` still present

`test/e2e/init.sh`
- write `/tmp/dotd.yaml` with `dotfiles: /tmp/testdotfiles\n`; `mkdir /tmp/testdotfiles`
- `dotd init --config /tmp/dotd.yaml`
- assert: `.dagger` file exists under `/tmp/testdotfiles/shellrc/` (or whichever dirs init scaffolds)

**Integration tests** (new in `integration_test.go`):

`TestSetupThenTeardown`
- set `XDG_CONFIG_HOME` to `t.TempDir()`; set `DOTFILES` to a temp dir so default dotfiles path exists
- create `newRootCmd()`, set stdin to `strings.Repeat("\n", 10)` (accept all defaults), run `setup`
- assert: config.yaml written at expected XDG path
- create `newRootCmd()`, set stdin to `"y\n"`, run `teardown`
- assert: config.yaml absent

`TestInitAfterSetup`
- same setup preamble as above to write config.yaml
- run `init` (non-interactive) via `newRootCmd()`
- assert: `.dagger` scaffolded in dotfiles dir

**Dockerfiles:** add `COPY setup.exp`, `COPY teardown-confirm.exp`, `COPY teardown-cancel.exp`, `COPY init.sh`.

**Runner additions:**
```sh
run_expect_test setup.exp
run_expect_test teardown-confirm.exp
run_expect_test teardown-cancel.exp
run_test init.sh
```

---

### Batch 4 — `config` / `env`

**Shell e2e:**

`test/e2e/config-cmds.sh`
- write `/tmp/dotd.yaml` with `dotfiles: /fixture\n`
- `config show --config /tmp/dotd.yaml` — assert output contains `dotfiles`
- `config get --config /tmp/dotd.yaml dotfiles` — assert output is `/fixture`
- `config set --config /tmp/dotd.yaml dotfiles /fixture2`
- `config get --config /tmp/dotd.yaml dotfiles` — assert output is `/fixture2`

`test/e2e/env-cmds.sh`
- copy `/fixture/env.yaml` to `/tmp/env.yaml` (fixture is read-only; `env set` must write)
- `env show --env-file /tmp/env.yaml` — assert known keys from fixture appear
- `env get --env-file /tmp/env.yaml os` — assert value matches fixture
- `env set --env-file /tmp/env.yaml context staging`
- `env get --env-file /tmp/env.yaml context` — assert `staging`
- `env diff --env-file /tmp/env.yaml --env os=macos` — assert override appears in output

**Integration tests** (new in `integration_test.go`):

`TestConfigCmdsLifecycle`
- config show on missing config → exit 0, no error
- config set `dotfiles /tmp/x` → config get `dotfiles` returns `/tmp/x`

`TestEnvCmdsLifecycle`
- env show with fixture env.yaml → known keys present in output
- env set `context staging` → env get `context` returns `staging`
- env diff with `--env os=macos` → override appears in output

**Dockerfiles:** add `COPY config-cmds.sh`, `COPY env-cmds.sh`.

**Runner additions:**
```sh
run_test config-cmds.sh
run_test env-cmds.sh
```

---

### Batch 5 — `adopt`, `bundle`, `dag check`, `package` subcommands

**Shell e2e:**

`test/e2e/adopt.sh`
- copy fixture to writable temp dir (`cp -r /fixture /tmp/dotfiles`)
- create `/tmp/dotfiles/shellrc/newscript.sh` with a shebang and body
- `adopt /tmp/dotfiles/shellrc/newscript.sh --files /tmp/dotfiles --to shellrc`
- assert: `newscript.sh` present in `shellrc/`, `.dagger` present in `shellrc/`

`test/e2e/bundle.sh`
- `bundle aliases.sh --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
- `aliases.sh` depends on `base.sh` in the fixture; assert output contains content from `base.sh` (a known string from that file)

`test/e2e/dag-check.sh`
- `dotd dag check --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
- assert exit 0
- assert output contains `base.sh`, `path.sh`, `aliases.sh` (numbered lines)
- assert `base.sh` line number is lower than `path.sh` line number (ordering)

`test/e2e/package-check.sh`
- `package check --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
- assert output contains `fake-installed` and `installed`
- copy fixture to writable temp dir; write a script with `# @require(not-installable)` into shellrc
- `package generate` against writable copy — assert exit non-zero

**Integration tests** (new in `integration_test.go`):

`TestAdoptShellScript_Integration`
- in a fresh `ienv`, create a new `.sh` file outside the fixture; adopt it `--to shellrc`
- assert file exists at `shellrc/<name>.sh`; assert `.dagger` present in `shellrc/`

`TestBundleOutput_Integration`
- `bundle shellrc/aliases.sh` against the testdata fixture (linux/personal)
- `aliases.sh` depends on `base.sh`; assert output contains a known string from `base.sh`

Skip `package generate` hard-fail (already in `TestPackageRequireHardFail`) and `dag check` ordering (already in `TestDAGVerboseOrder`).

**Dockerfiles:** add `COPY adopt.sh`, `COPY bundle.sh`, `COPY dag-check.sh`, `COPY package-check.sh`.

**Runner additions:**
```sh
run_test adopt.sh
run_test bundle.sh
run_test dag-check.sh
run_test package-check.sh
```

---

## Delivery Order

| Batch | New shell e2e | New integration tests | Notes |
|-------|---------------|-----------------------|-------|
| 1 | `unapply.sh`, `unapply-cancel.exp` | `TestUnapplyAfterApply`, `TestUnapplyCancel` | Requires `runWithStdin` harness addition |
| 2 | `compose.sh`, `macos-apply.sh` + fixture extension | none | |
| 3 | `setup.exp`, `teardown-confirm.exp`, `teardown-cancel.exp`, `init.sh` | `TestSetupThenTeardown`, `TestInitAfterSetup` | |
| 4 | `config-cmds.sh`, `env-cmds.sh` | `TestConfigCmdsLifecycle`, `TestEnvCmdsLifecycle` | |
| 5 | `adopt.sh`, `bundle.sh`, `dag-check.sh`, `package-check.sh` | `TestAdoptShellScript_Integration`, `TestBundleOutput_Integration` | |

Each batch is independently mergeable. Batch 1 is highest priority (largest coverage gap, most-used recovery workflow).
