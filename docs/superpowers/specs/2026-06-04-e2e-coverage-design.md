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

- **Shell e2e** — real binary in Docker, all `.sh`
- **Go integration tests** — in-process via `//go:build integration`, in `integration_test.go`

Exception: where existing integration tests already give thorough coverage (compose, dag check, package generate), shell e2e is added but no new Go integration tests are written.

## Interactive Commands — No Expect Needed

`promptConfirm`, `promptYN`, and `promptDefault` (used by unapply, setup, init, teardown) have no TTY check — they read from `cmd.InOrStdin()` directly. Piped stdin works in all shell scripts:

```sh
printf 'n\n' | dotd unapply ...   # cancel path
printf 'y\n' | dotd teardown ...  # confirm path
```

`adopt` is the only command that calls `isTTYStdin()`. When stdin is not a TTY (Docker), it auto-skips the huh confirmation prompt and runs non-interactively. No expect required there either.

**Expect is not needed for any command in this plan.** All interactive shell e2e tests use piped stdin.

## Infrastructure Changes

### Docker (`Dockerfile` and `Dockerfile.local`)

No new packages required. Add explicit `COPY` lines for each new `.sh` file alongside existing ones. Both Dockerfiles use per-file COPY lines, so each new script needs its own line.

### `test/run-e2e.sh`

No new runner function needed — all new scripts use the existing `run_test`.

### `integration_test.go` — harness extension

Add `runWithStdin` to `ienv` to support piped stdin for `TestUnapplyCancel`:

```go
func (e *ienv) runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
    t.Helper()
    base := []string{
        "--files", e.dotfiles,
        "--env-file", filepath.Join(e.dotfiles, "env.yaml"),
        "--link-root", e.home,
        "--bin-dir", e.binDir,
        "--init-file", e.initFile,
        "--generated-dir", e.generatedDir,
    }
    var buf bytes.Buffer
    cmd := newRootCmd()
    cmd.SetIn(stdin)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    cmd.SetArgs(append(base, args...))
    return buf.String(), cmd.Execute()
}
```

Setup/teardown integration tests call `newRootCmd()` directly (same pattern as unit tests) since they manage their own `XDG_CONFIG_HOME`, not `ienv` paths.

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

`test/e2e/unapply-cancel.sh`
- apply (linux/personal)
- `printf 'n\n' | dotd unapply ...` (no `--yes`)
- assert: exit 0, `.zshrc` symlink still present

**Integration tests** (new in `integration_test.go`):

`TestUnapplyAfterApply`
- apply (linux/personal), then `unapply --yes`
- assert: symlinks gone, `initFile` absent

`TestUnapplyCancel`
- apply (linux/personal)
- call `runWithStdin(t, strings.NewReader("n\n"), "unapply")`
- assert: exit 0, symlinks preserved

**Dockerfiles:** add `COPY unapply.sh /tests/unapply.sh` and `COPY unapply-cancel.sh /tests/unapply-cancel.sh`.

**Runner additions:**
```sh
run_test unapply.sh
run_test unapply-cancel.sh
```

---

### Batch 2 — `compose` + macOS predicate

**Fixture extension** (described in Infrastructure section above).

**Shell e2e:**

`test/e2e/compose.sh`
- apply (linux/personal)
- assert: `/tmp/generated/extras.sh` exists and is sourced in `/tmp/init.sh`
- `compose list` — assert `extras.sh` appears in output
- `compose check` — assert exit 0 (clean state)
- overwrite `/tmp/generated/extras.sh` with stale content
- `compose check` — assert output contains "stale"

`test/e2e/macos-apply.sh`
- apply `--env os=macos --env context=personal` (Linux binary, macOS predicate via flag)
- assert: `macos.sh` in `/tmp/init.sh`, `linux.sh` absent

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

`test/e2e/setup.sh`
- `export XDG_CONFIG_HOME=/tmp/xdg`
- `export DOTFILES=/fixture` — pins the default dotfiles path so `resolvePaths` resolves cleanly and the assertion is deterministic
- `printf '\n\n\n\n' | dotd setup` — 4 newlines accept all 4 prompts (dotfiles, bin dir, generated dir, link root) with defaults
- assert: `/tmp/xdg/dot-dagger/config.yaml` exists and contains `dotfiles: /fixture`

`test/e2e/teardown-confirm.sh`
- `export XDG_CONFIG_HOME=/tmp/xdg; mkdir -p /tmp/xdg/dot-dagger`
- write `/tmp/xdg/dot-dagger/config.yaml` with `dotfiles: /fixture\n`
- `printf 'y\n' | dotd teardown --files /fixture --env-file /fixture/env.yaml`
- assert: `/tmp/xdg/dot-dagger/config.yaml` absent after exit
- Note: `teardown` always removes `dotcfg.DefaultPath()` (which respects `XDG_CONFIG_HOME`), not the `--config` flag path. `--files` / `--env-file` are still required for the internal pipeline check.

`test/e2e/teardown-cancel.sh`
- same setup as teardown-confirm
- `printf 'n\n' | dotd teardown --files /fixture --env-file /fixture/env.yaml`
- assert: exit 0, `/tmp/xdg/dot-dagger/config.yaml` still present

`test/e2e/init.sh`
- `export XDG_CONFIG_HOME=/tmp/xdg`; write `/tmp/xdg/dot-dagger/config.yaml` with `dotfiles: /tmp/testdotfiles\n`; `mkdir /tmp/testdotfiles`
- `printf 'y\n\ny\n\ny\n\n' | dotd init --config /tmp/xdg/dot-dagger/config.yaml`
  - 3 × (`y\n` = create this dir + `\n` = accept default name): shellrc, config, bin
  - trailing EOF safely defaults the optional source-line prompt to "no"
- assert: `.dagger` exists under `/tmp/testdotfiles/shellrc/`, `/tmp/testdotfiles/config/`, `/tmp/testdotfiles/bin/`

**Integration tests** (new in `integration_test.go`):

`TestSetupThenTeardown`
- `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` and `t.Setenv("DOTFILES", t.TempDir())`
- `newRootCmd()`, stdin `strings.Repeat("\n", 10)`, run `setup` — assert config.yaml written
- `newRootCmd()`, stdin `"y\n"`, run `teardown --files <emptyDotfiles> --env-file <emptyEnvFile>` — assert config.yaml absent

`TestInitAfterSetup`
- same XDG preamble; run setup (accept defaults)
- run `init` with stdin `"y\n\ny\n\ny\n\nn\n"` via `newRootCmd()` — 3 × (yes + accept default name) then explicit "n" to the source-line prompt; avoids `strings.Repeat("\n", 10)` which would answer yes to the source-line prompt and attempt to modify a shell RC file
- assert: `.dagger` scaffolded in each of `shellrc/`, `config/`, `bin/` under dotfiles dir

**Dockerfiles:** add `COPY setup.sh`, `COPY teardown-confirm.sh`, `COPY teardown-cancel.sh`, `COPY init.sh`.

**Runner additions:**
```sh
run_test setup.sh
run_test teardown-confirm.sh
run_test teardown-cancel.sh
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
- `cp /fixture/env.yaml /tmp/env.yaml` (fixture is read-only; `env set` must write)
- `env show --env-file /tmp/env.yaml` — assert known keys from fixture appear
- `env get --env-file /tmp/env.yaml os` — assert value matches fixture
- `env set --env-file /tmp/env.yaml context staging`
- `env get --env-file /tmp/env.yaml context` — assert `staging`
- `env diff --env-file /tmp/env.yaml --env os=macos` — assert override appears in output

**Integration tests** (new in `integration_test.go`):

`TestConfigCmdsLifecycle`
- config show on missing config file → exit 0, no error
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
- copy fixture to writable temp dir: `cp -r /fixture /tmp/dotfiles`
- create source file **outside** dotfiles dir: `printf '#!/bin/sh\necho hi\n' > /tmp/newscript.sh`
- `adopt /tmp/newscript.sh --files /tmp/dotfiles --to shellrc/`
  - `--to shellrc/` (trailing slash) appends filename → destination `shellrc/newscript.sh`
  - no TTY in Docker → `isTTYStdin()` false → huh prompt auto-skipped
- assert: `/tmp/dotfiles/shellrc/newscript.sh` exists
- assert: `/tmp/newscript.sh` absent (moved, not copied)

`test/e2e/bundle.sh`
- `bundle shellrc/aliases.sh --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
  - `aliases.sh` has a transitive dep on `base.sh`; bundle inlines all deps' contents
- assert: output contains a known string from `base.sh` content (a string unique to that file)

`test/e2e/dag-check.sh`
- `dotd dag check --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
- assert: exit 0
- capture output; assert lines contain `base.sh`, `path.sh`, `aliases.sh`
- assert: line number for `base.sh` is lower than line number for `path.sh` (verify ordering)

`test/e2e/package-check.sh`
- `package check --files /fixture --env-file /fixture/env.yaml --env os=linux --env context=personal`
- assert: output contains `fake-installed` and `installed`
- copy fixture to writable temp dir; write a script with `# @require(not-installable)` into shellrc
- `package generate --files /tmp/dotfiles --env-file /tmp/dotfiles/env.yaml --env os=linux` — assert exit non-zero

**Integration tests** (new in `integration_test.go`):

`TestAdoptShellScript_Integration`
- using a fresh `ienv`
- create a new `.sh` file in a **separate** `t.TempDir()` (outside `e.dotfiles`): `srcPath := filepath.Join(t.TempDir(), "newscript.sh")`
- `e.run(t, "adopt", srcPath, "--to", "shellrc/")` (no TTY in tests → prompt auto-skipped)
- assert: `filepath.Join(e.dotfiles, "shellrc", "newscript.sh")` exists
- assert: `srcPath` absent (moved)

`TestBundleOutput_Integration`
- `bundle shellrc/aliases.sh` against the testdata fixture (linux/personal)
- assert: output contains a known string from `base.sh` content in the testdata fixture

Skip `package generate` hard-fail (already `TestPackageRequireHardFail`) and `dag check` ordering (already `TestDAGVerboseOrder`).

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
| 1 | `unapply.sh`, `unapply-cancel.sh` | `TestUnapplyAfterApply`, `TestUnapplyCancel` | Requires `runWithStdin` harness addition |
| 2 | `compose.sh`, `macos-apply.sh` + fixture extension | none | |
| 3 | `setup.sh`, `teardown-confirm.sh`, `teardown-cancel.sh`, `init.sh` | `TestSetupThenTeardown`, `TestInitAfterSetup` | Teardown targets `DefaultPath()`, not `--config` |
| 4 | `config-cmds.sh`, `env-cmds.sh` | `TestConfigCmdsLifecycle`, `TestEnvCmdsLifecycle` | |
| 5 | `adopt.sh`, `bundle.sh`, `dag-check.sh`, `package-check.sh` | `TestAdoptShellScript_Integration`, `TestBundleOutput_Integration` | Adopt auto-skips prompt in non-TTY |

Each batch is independently mergeable. Batch 1 is highest priority.
