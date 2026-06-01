# UX and CLI Behavior

CLI-convention defects: stream usage (stdout vs stderr), exit codes, prompt consistency, and help text.

### [AUDIT-033] `unapply` reports remove failures to stdout and still exits 0

**Original ID:** F-002
**Location:** `cmd/dotd/unapply_cmd.go:46` (`out := cmd.OutOrStdout()`), `:136-149` (remove loop), `:151` (`return nil`)
**Severity:** High
**Description:** In the removal loop, a failed `os.Remove` is reported via `ui.Errf(out, ...)` (where `out = cmd.OutOrStdout()`) and the loop `continue`s; the function returns `nil` regardless. The command exits 0 even when one or more symlinks (or `init.sh`) failed to delete. Two violations at once: (1) the error text goes to stdout instead of stderr; (2) the exit code is 0 despite partial failure.
**Justification:** Confirmed silent-failure / wrong-stream defect: partial failure exits 0 with the error in captured stdout.
**Impact:** A scripted `dotd unapply` that fails to remove some links looks successful (`$? == 0`) and the error lands in captured stdout — silent failure for automation.
**Cross-reference:** AUDIT-034.

### [AUDIT-034] Warnings and errors written to stdout instead of stderr in several commands

**Original ID:** F-003
**Location:** `cmd/dotd/teardown_cmd.go:48,54`; `cmd/dotd/unapply_cmd.go:138,145`; `cmd/dotd/compose_cmd.go:94,102` (vs summary on `cfg.log`/stderr at `:108,110`)
**Severity:** Medium
**Description:** `internal/log/log.go:2-3` documents the contract: status/diagnostic output goes to stderr, data output to stdout. Multiple commands route diagnostic-class output (`ui.Warnf`, `ui.Errf`, stale/missing markers) to `cmd.OutOrStdout()`. `compose check` is internally split: `ui.Missingf`/`ui.Wrongf` markers go to stdout while the summary line goes to `cfg.log` (stderr).
**Justification:** Diagnostic-class output routes to stdout and compose check splits status across both streams, contradicting the documented contract. Interactive prompts/preview legitimately use stdout, but warning/error labels are diagnostics.
**Impact:** Diagnostic noise contaminates pipeable stdout; `2>/dev/null` does not suppress warnings as expected. Stream usage is internally inconsistent.
**Cross-reference:** AUDIT-033.

### [AUDIT-035] Three confirmation-prompt mechanisms with inconsistent defaults and non-TTY behavior

**Original ID:** F-004
**Location:** `cmd/dotd/prompts.go:15-24` (`promptConfirm`, `[y/N]` default-No), `cmd/dotd/init_cmd.go:164-172` (`promptYN`, `[Y/n]` default-Yes, EOF→yes), `cmd/dotd/adopt.go:160-172` & `cmd/dotd/filter_prompt.go:54-76` (huh forms requiring a TTY)
**Severity:** Medium
**Description:** The CLI uses three unrelated prompt implementations with opposite default capitalization and differing non-TTY behavior. `promptConfirm` (teardown/unapply) defaults No and empty input cancels; `promptYN` (init) defaults Yes, empty input proceeds, and EOF also returns yes; the huh forms require a TTY to render.
**Justification:** The `[y/N]` vs `[Y/n]` split is a genuine UX inconsistency, but the more serious divergence is non-TTY handling: `promptYN` treats EOF as "yes," so `init` piped from a closed/empty stdin silently auto-accepts every directory-creation prompt.
**Impact:** Inconsistent muscle memory (Enter means "yes" in `init`, "cancel" in `teardown`/`unapply`); `init` from closed stdin silently auto-accepts everything.
**Cross-reference:** AUDIT-023, AUDIT-018.

### [AUDIT-036] `config edit` help text names the wrong file

**Original ID:** F-006
**Location:** `cmd/dotd/config_cmd.go:90` (`Short: "Open dotcfg.yaml in $EDITOR"`), `:96` (opens `cfg.configPath`)
**Severity:** Low
**Description:** The `Short` for `config edit` reads "Open dotcfg.yaml in $EDITOR" but the file opened is `cfg.configPath`, i.e. `config.yaml`. There is no file named `dotcfg.yaml` — `dotcfg` is the Go import alias for the config package. `dotd env edit` correctly says "Open env.yaml".
**Justification:** Help text references a filename that does not exist on disk; mildly confusing when users look for `dotcfg.yaml`.
**Impact:** Minor doc/help defect; users may search for a nonexistent file.
