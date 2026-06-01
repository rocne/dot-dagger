# Magic Values and Constants

Hardcoded strings/numbers/paths duplicated across files with no shared constant, where a one-sided change would silently break behavior.

### [AUDIT-004] `packages.yaml` filename built by independent `filepath.Join` calls in reader vs writer

**Original ID:** B-002
**Location:** `cmd/dotd/package.go:134` (reader: `filepath.Join(cfg.files, "packages.yaml")`), `internal/setup/setup.go:78` (writer: `filepath.Join(opts.DotfilesDir, "packages.yaml")`)
**Severity:** High
**Description:** Unlike `env.yaml`/`config.yaml`, the registry filename `packages.yaml` has no constant at all. It is computed by two independent literal `filepath.Join` calls in different packages: `package.go:134` (where `dotd package` reads it) and `setup.go:78` (where `dotd setup` scaffolds it). There is no `ecosystem.PackagesFile` or `packages.DefaultName`.
**Justification:** Reader and writer each spell the filename literally with no shared source, so they can desync.
**Impact:** A one-sided rename means `dotd setup` writes a file `dotd package` never reads (or vice versa); `LoadFile` returns an empty registry for a missing file (packages.go:136), so installs become silent no-ops with no error.

### [AUDIT-005] `{package}` substitution token duplicated between catalog defaults and substitution logic

**Original ID:** B-003
**Location:** `internal/packages/packages.go:239` (consumer: `strings.ReplaceAll(mgDef.Install, "{package}", pkgName)`), `internal/packages/catalog.go:18-101` (24 catalog literals embedding `{package}`)
**Severity:** High
**Description:** The placeholder token `{package}` is the contract between catalog command templates and the substitution call. It is defined nowhere as a constant — the literal `"{package}"` is hardcoded in `ReplaceAll` and embedded in 24 catalog command strings (dnf/yum/apt/pacman/brew/zypper/apk/cargo/npm/pip3 install/uninstall/update).
**Justification:** The token in `ReplaceAll` must match exactly what the catalog and authors write. (Note: the source finding's prose at packages.go:77-78 is garbled, but the cited code and impact are correct.)
**Impact:** A rename on one side (e.g. `{pkg}`) makes substitution no-op, so the shell executes `dnf install -y {package}` literally — attempting to install a package named `{package}`.

### [AUDIT-006] Annotation/`.dagger` key vocabulary triplicated: `ann*` constants vs YAML tags vs bare literal

**Original ID:** B-004
**Location:** `internal/pipeline/walk.go:26-31` (`annAfter="after"`, `annRequire="require"`, `annRequest="request"`, `annDisable="disable"`, `annName="name"`, `annAction="action"`), `internal/dagger/dagger.go:17-23,30` (`yaml:"when"/"after"/"require"/"request"/"disable"/"name"`), `internal/annotation/annotation.go:121` (bare `a.Key == "when"`)
**Severity:** High
**Description:** The annotation vocabulary is the shared contract between the comment-annotation parser, the `.dagger` YAML parser, and the pipeline walker. It is expressed three ways: `ann*` constants in `walk.go`, raw `yaml:"..."` tags in `dagger.go`, and a bare `"when"` literal in `annotation.go`. Both the `# @after(...)` path and the `.dagger` `after:` path must yield the same logical key for `annotation.Get(anns, annAfter)` to find them.
**Justification:** `annAfter="after"` and `yaml:"after"` are two independent declarations of the same string. (Note: triplication is exact for after/require/request/disable/name; slightly looser for action/when — `annAction` has no `dagger.go` tag analog, `When` has no `annWhen` constant — but the core risk stands.)
**Impact:** Renaming `after` requires lockstep edits across `walk.go`, `dagger.go`, and the parser; a one-sided change (e.g. `yaml:"depends"`) silently drops `.dagger`-declared DAG edges while comment-style `@after` keeps working — edges vanish with no error. Highest-risk magic-value cluster.
**Cross-reference:** AUDIT-013.

### [AUDIT-007] Reserved package keys duplicated as struct tags AND bare literals in custom unmarshalers

**Original ID:** B-005
**Location:** `internal/packages/packages.go:57` (bare `if key == "priority"`), `internal/packages/packages.go:89-91` (tags `yaml:"binary"/"check"/"prefer"`), `internal/packages/packages.go:102` (`known := map[string]bool{"binary":true,"check":true,"prefer":true}`)
**Severity:** High
**Description:** Because `ManagersSection` and `PackageEntry` use hand-written `UnmarshalYAML` that treat unknown keys as manager defs/entries, they must explicitly filter the known keys. Those names are written both as struct tags (89-91) and again as bare literals (line 57 `"priority"`, line 102 the `known` map). The `known` map exists solely to skip the same fields the struct tags declare.
**Justification:** If tag and `known` map diverge, a known field is reprocessed as a package-manager entry.
**Impact:** Renaming `yaml:"binary"` → `yaml:"bin"` without updating the `known` map means `binary:` in a user's packages.yaml is decoded as a package manager named "binary" — silently corrupting the registry.

### [AUDIT-008] Convention directory names (`shellrc`, `bin`, `config`) duplicated across four files

**Original ID:** B-006
**Location:** `internal/adopter/adopter.go:23` (`ConventionNames{Shellrc:"shellrc", Bin:"bin", Config:"config"}`), `internal/setup/setup.go:63` (`[]string{"shellrc","config","bin"}`), `cmd/dotd/init_cmd.go:77,83,89` (`defDir:"shellrc"/"config"/"bin"`), `internal/dagger/dagger.go:41-43` (`yaml:"shellrc"/"bin"/"config"`)
**Severity:** High
**Description:** The three convention dir names are the contract between scaffolding (`init_cmd`, `setup`), adoption inference (`adopter`), and the `.dagger` convention-override parser. Each spells the names as independent literals; there is no single `Convention*` constant.
**Justification:** These four sites in unrelated packages must agree on the default names, yet nothing enforces it. The lists are unordered, making divergence easy.
**Impact:** If `setup` creates a `shell/` dir but `adopter`/walk still expect `shellrc/`, scaffolded files land where the pipeline never looks — sourcing/linking silently stops working.

### [AUDIT-009] `env.yaml` filename hardcoded in ~15 user-facing hint strings

**Original ID:** B-001
**Location:** `internal/ecosystem/ecosystem.go:66` (canonical builder), plus literal `env.yaml` in copy: `cmd/dotd/main.go` (4×), `cmd/dotd/env.go` (8×), `cmd/dotd/setup_cmd.go` (3×), `cmd/dotd/filter_prompt.go` (2×)
**Severity:** Medium
**Description:** The on-disk filename `env.yaml` is produced canonically only at `ecosystem.go:66` (functional path is correct), but the literal `"env.yaml"` recurs in ~15 help/hint strings telling users to "add to env.yaml".
**Justification:** Copy-only divergence: a rename in `ecosystem.go` leaves these hints pointing users at a nonexistent file. (Note: borderline — no correctness/compile impact; Low would be defensible, but the surfacing is valid.)
**Impact:** Filename rename silently leaves stale instructions pointing at a file that no longer exists. No compile error or test failure unless tests assert the exact strings.

### [AUDIT-010] `DOTD_` env-var prefix written twice on adjacent lines

**Original ID:** B-009
**Location:** `internal/env/env.go:125` (`strings.HasPrefix(e, "DOTD_")`), `internal/env/env.go:128` (`rest := e[len("DOTD_"):]`)
**Severity:** Medium
**Description:** The `DOTD_` namespace prefix is written twice in adjacent lines of the `ShellVars` extractor — once in `HasPrefix` and once as the slice length. (The `main.go` full-name vars are a separate scheme and correctly excluded.)
**Justification:** Two adjacent `"DOTD_"` occurrences (prefix check + slice length) are a classic change-one-miss-one hazard.
**Impact:** If the prefix is changed in one without the other, `ShellVars` either matches the wrong vars or slices the key incorrectly (e.g. keeping a leading `_`) — env keys silently mangled.

### [AUDIT-011] File-mode literals `0o755` / `0o644` repeated ~16 times with no named constant

**Original ID:** B-007
**Location:** `0o755`: `cmd/dotd/init_cmd.go:134`, `cmd/dotd/setup_cmd.go:144`, `internal/pipeline/act.go:170,246`, `internal/pipeline/initgen.go:41`, `internal/adopter/adopter.go:169`, `internal/fileutil/fileutil.go:15`, `internal/setup/setup.go:95,105`; `0o644`: `cmd/dotd/bundle.go:121`, `cmd/dotd/init_cmd.go:137`, `cmd/dotd/setup_cmd.go:148`, `internal/pipeline/act.go:173`, `internal/setup/shell.go:89,129`, `internal/setup/setup.go:108`
**Severity:** Low
**Description:** The permission bits `0o755` (dirs/scripts) and `0o644` (files) are repeated as bare octal literals across ~7 files / ~16 sites with no shared constant.
**Justification:** dir=755/file=644 is universal convention and no site is load-bearing against another, so a one-sided change is a consistency risk, not correctness. Pure hygiene — surfaced only because a future hardening change (e.g. `0o600`/`0o700`) would need to touch every site.
**Impact:** Inconsistent permissions if a security-hardening change misses a site. Cosmetic/security-hygiene, not functional breakage.

### [AUDIT-012] Generated-script shebang `#!/bin/sh` duplicated across two generators

**Original ID:** B-008
**Location:** `cmd/dotd/bundle.go:90` (`sb.WriteString("#!/bin/sh\n")`), `internal/packages/packages.go:281` (`fmt.Fprintln(w, "#!/bin/sh")`)
**Severity:** Low
**Description:** Two independent script generators (bundle writer, package install-script generator) each emit the POSIX shebang as a separate literal.
**Justification:** Both must stay POSIX-compatible; a shared constant would document the interpreter contract, but the value is a near-universal convention and divergence is near-harmless (both are valid sh shebangs).
**Impact:** Minimal. If one generator switched interpreters intentionally, the other would not follow, but each script is self-contained.
