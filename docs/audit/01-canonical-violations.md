# Canonical Resolution Violations

Findings where a value with a canonical resolution path (`resolvePaths`/`ecosystem.ResolvePath`, `resolveEnv`) is re-derived or bypassed, violating the single-source rule in CLAUDE.md.

### [AUDIT-001] `cfg.linkRoot` `$HOME` default re-derived in three consumers, only one correct

**Original ID:** A-001
**Location:** `cmd/dotd/main.go:222` (resolution with no-op default fn), `cmd/dotd/main.go:257-265` (`buildActOptions` fills `os.UserHomeDir()`), `cmd/dotd/adopt.go:113` → `internal/adopter/adopter.go:115` → `internal/pipeline/act.go:42-44`, `cmd/dotd/teardown_cmd.go:75` → `internal/setup/shell.go:25-34`
**Severity:** Critical
**Description:** `linkRoot` is the one path field whose `$HOME` default is *not* applied in `resolvePaths()` — line 222 passes a no-op default fn `func() (string, error) { return "", nil }`, so with no `--link-root`/`DOTD_LINK_ROOT`/config field (the common case) `cfg.linkRoot == ""`. The `$HOME` fallback the flag help advertises is then applied independently by each consumer, and only one of three actually applies it: `buildActOptions` fills `os.UserHomeDir()` (correct); `adopt.go:113` passes the raw empty value into `ActOptions.HomeDir`, which hard-errors at `act.go:42-44` ("act: HomeDir is required"); `teardown_cmd.go:75` passes the empty value into `setup.DetectShellConfig`, resolving `.bashrc`/`.zshrc` relative to the process cwd instead of `$HOME`.
**Justification:** This is the exact CLAUDE.md anti-pattern — a default with a canonical home (`resolvePaths`) is re-derived ad hoc in three downstream sites, and two derivations are wrong. The same default-config user gets three different notions of "home."
**Impact:** Identical user config, divergent behavior: `apply` works, `adopt` errors out, `teardown` looks for RC files in the wrong directory and silently fails to strip the source line. Changing how `linkRoot` defaults requires editing three sites — the textbook silent-breakage case.

### [AUDIT-002] `config.yaml` path bypasses `ResolvePath` — no flag or env-var tier

**Original ID:** A-002
**Location:** `cmd/dotd/main.go:201` (`cfg.configPath, err = dotcfg.DefaultPath()`), `internal/config/config.go:24-26` (`DefaultPath` → `ecosystem.DefaultConfigFile`)
**Severity:** High
**Description:** Every other path field is resolved through `ecosystem.ResolvePath` with the full precedence chain (CLI flag → `DOTD_*` → config field → default). `cfg.configPath` alone is set straight from `dotcfg.DefaultPath()`. There is no `--config` flag, no `DOTD_CONFIG_FILE` env var, and no override path — config.yaml can only ever live at the XDG default.
**Justification:** CLAUDE.md mandates paths be resolved via `ResolvePath`'s flag/env/config/default chain. `configPath` is resolved in `resolvePaths` but bypasses `ResolvePath` entirely, so it silently lacks the flag/env tiers that `envFile`, `files`, `initFile`, `linkRoot`, `binDir`, and `generatedDir` all have — a structural asymmetry, not just a missing feature.
**Impact:** config.yaml is non-relocatable while env.yaml is. Tests and multi-profile setups can override every path but this one. The fix is mechanical (route through `ResolvePath` plus a flag) but until then it surprises anyone extending config handling.

### [AUDIT-003] `setup` and `init` re-call `dotcfg.DefaultPath()` instead of reading `cfg.configPath`

**Original ID:** A-003
**Location:** `cmd/dotd/setup_cmd.go:45`, `cmd/dotd/init_cmd.go:36`
**Severity:** Medium
**Description:** `resolvePaths` runs in `PersistentPreRunE` (main.go:73-74) before any subcommand `RunE` and stores the resolved config path in `cfg.configPath` (main.go:201) "so config subcommands don't re-resolve it." The `config` subcommands honor this, but `setup` and `init` re-derive it via `dotcfg.DefaultPath()`. init_cmd's "bootstrap check that runs before any config is loaded" comment is inaccurate — `resolvePaths` has already loaded config and set `cfg.configPath` before `runInit` runs.
**Justification:** `cfg.configPath` is the canonical resolved value, guaranteed populated by the time these RunE functions execute. Calling `DefaultPath()` again is a redundant raw lookup of a value with an existing resolved home.
**Impact:** Harmless today only because AUDIT-002 leaves `configPath` un-overridable. The moment AUDIT-002 is fixed (config gets `--config`/`DOTD_CONFIG_FILE`), `setup`/`init` would write to / check the default location while the rest of the CLI uses the overridden one — a latent divergence baked in now.
**Cross-reference:** AUDIT-002.
