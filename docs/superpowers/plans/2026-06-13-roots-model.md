# Roots Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kill the global `link_root` knob so `~` always means `$HOME`, and add a configurable `config_dir` route via the `$config` token; rename the tool's own state-file flags to `--dotd-config`/`--dotd-env`.

**Architecture:** Three anchor tokens in `.dagger` `link_root:` values — `~` (real `$HOME`, never configurable), `$bin` (bin dir), `$config` (config dir, new). The pipeline stays a pure function: all three anchors are resolved at the cmd layer and injected into `ActOptions`. Home is read on demand via a new `ecosystem.Home()` accessor (no flag/env/config tier, no `cfg.home` field). No backward compatibility — the `LinkRoot` config field and old flag names are removed cleanly.

**Tech Stack:** Go 1.26, cobra/pflag, yaml.v3 (strict `KnownFields`), testify-free table tests.

**Spec:** `docs/superpowers/specs/2026-06-13-roots-model-design.md`

**Ordering note:** Do tasks in order. Task 1 changes the `BinPrefix` const value (`~bin` → `$bin`), which fixtures (Task 9) depend on; the full suite only goes green after Task 9. Between tasks, expect *targeted* test breakage in not-yet-updated areas — that is fine; each task lists exactly what it must make pass.

---

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `internal/pipeline/act.go` | Token expansion (`expandDest`), `ActOptions` | Add `$config` branch + `ConfigDir`; rename `BinPrefix`→`$bin`; add `ConfigPrefix` |
| `internal/pipeline/actions.go` | `CheckLinkConflicts` call into `resolveLink` | Thread `ConfigDir` |
| `internal/config/config.go` | config.yaml schema/get/set | Drop `LinkRoot`; add `ConfigDir` |
| `internal/ecosystem/ecosystem.go` | path/home defaults | Add `Home()`; remove `DefaultLinkRoot` |
| `cmd/dotd/main.go` | flags, `resolvePaths`, `buildActOptions`, cfg struct | Rename/drop/add flags; drop `linkRoot`, add `configDir`; `buildActOptions` returns error |
| `cmd/dotd/setup_cmd.go` | setup wizard | Drop "Link root" prompt; add "Config directory" prompt |
| `cmd/dotd/init_cmd.go` | scaffold + source-line | `$config`/`$bin` scaffold; `ecosystem.Home()` |
| `cmd/dotd/teardown_cmd.go` | teardown | `ecosystem.Home()` |
| `cmd/dotd/adopt.go` + `internal/adopter/adopter.go` | adopt | `ecosystem.Home()`; rename `AdoptOptions.LinkRoot`→`HomeDir`, add `ConfigDir` |
| `cmd/dotd/config_cmd.go` | config subcommand help | Examples `link_root`→`config_dir` |
| testdata/e2e fixtures + tests | encode behavior | `~bin`→`$bin`; `--link-root`→env config |
| docs | user-facing | reference/concepts/spec sections |

---

## Task 1: Pipeline token expansion (`$bin`, `$config`, `~`)

**Files:**
- Modify: `internal/pipeline/act.go` (`ActOptions` ~14-24, `resolveLink` ~178-183, `expandDest` ~217-230, `Act` error msg ~51)
- Modify: `internal/pipeline/actions.go:69` (resolveLink call)
- Test: `internal/pipeline/act_test.go`

- [ ] **Step 1: Write the failing test** — add to `internal/pipeline/act_test.go`:

```go
func TestExpandDest_Anchors(t *testing.T) {
	const home, bin, conf = "/home/u", "/home/u/.local/bin/dot-dagger", "/home/u/.config"
	cases := []struct{ in, want string }{
		{"~", home},
		{"~/.zshrc", home + "/.zshrc"},
		{"$bin", bin},
		{"$bin/fmt", bin + "/fmt"},
		{"$config", conf},
		{"$config/nvim/init.lua", conf + "/nvim/init.lua"},
		{"/abs/path", "/abs/path"},
		{"relative/path", "relative/path"},
	}
	for _, c := range cases {
		if got := expandDest(c.in, home, bin, conf); got != c.want {
			t.Errorf("expandDest(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -run TestExpandDest_Anchors`
Expected: FAIL — `expandDest` currently takes 3 args (`path, homeDir, binDir`), so this is a compile error ("too many arguments").

- [ ] **Step 3: Update the token constants and `ActOptions`** in `internal/pipeline/act.go`:

Replace lines 14-24 (the `ActOptions` struct and `BinPrefix` const) with:

```go
// ActOptions configures Act behaviour.
type ActOptions struct {
	HomeDir      string // replaces "~" in link destinations; the real $HOME
	BinDir       string // replaces "$bin" in link destinations
	ConfigDir    string // replaces "$config" in link destinations
	GeneratedDir string // directory for compose-generated files
	DryRun        bool  // validate without writing to filesystem
	Force         bool  // overwrite non-symlink files on link conflicts
}

// BinPrefix and ConfigPrefix are the destination tokens that expand to
// ActOptions.BinDir / ActOptions.ConfigDir ("$bin", "$config", optionally
// followed by "/<rel>"). "~" expands to HomeDir (the real $HOME) and is never
// configurable.
const (
	BinPrefix    = "$bin"
	ConfigPrefix = "$config"
)
```

- [ ] **Step 4: Rewrite `expandDest`** in `internal/pipeline/act.go` (replace the function at ~217-230):

```go
// expandDest expands the "~", "$bin", and "$config" anchor tokens in a link
// destination. "~" / "~/x" go through fileutil.ExpandHome (real $HOME);
// "$bin" / "$config" (optionally "/<rel>") map to binDir / configDir.
// Absolute and relative paths are returned unchanged.
func expandDest(path, homeDir, binDir, configDir string) string {
	if path == "~" || (len(path) >= 2 && path[0] == '~' && path[1] == '/') {
		return fileutil.ExpandHome(path, homeDir)
	}
	if v := expandToken(path, BinPrefix, binDir); v != "" {
		return v
	}
	if v := expandToken(path, ConfigPrefix, configDir); v != "" {
		return v
	}
	return path
}

// expandToken returns base (optionally joined with the "/<rel>" suffix) when
// path is exactly token or token+"/...". Returns "" when path does not match
// the token or base is empty (token unconfigured).
func expandToken(path, token, base string) string {
	if base == "" {
		return ""
	}
	if path == token {
		return base
	}
	if strings.HasPrefix(path, token+"/") {
		return filepath.Join(base, path[len(token)+1:])
	}
	return ""
}
```

- [ ] **Step 5: Thread `configDir` through `resolveLink`** in `internal/pipeline/act.go` (~178-183):

```go
func resolveLink(dest string, n RawNode, homeDir, binDir, configDir string) string {
	if dest == "" {
		dest = deriveLinkDest(n)
	}
	return expandDest(dest, homeDir, binDir, configDir)
}
```

And update its caller at `act.go:168`:

```go
			dest := resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```

And the caller in `internal/pipeline/actions.go:69`:

```go
				dest = resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```

- [ ] **Step 6: Fix the `Act` error message** in `act.go:51` (remove the dead `cfg.linkRoot` reference):

```go
		return nil, fmt.Errorf("act: HomeDir is required — set it to the resolved $HOME")
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/pipeline/`
Expected: PASS for `TestExpandDest_Anchors`. NOTE: other pipeline tests that hardcode `~bin` in expected data will FAIL — that is expected; they are fixed in Task 9. If any failure is *not* about `~bin`/`$bin`, stop and investigate.

- [ ] **Step 8: Commit**

```bash
git add internal/pipeline/act.go internal/pipeline/actions.go internal/pipeline/act_test.go
git commit -m "feat(pipeline): add \$config anchor, move \$bin off ~bin token"
```

---

## Task 2: config.yaml schema — drop `link_root`, add `config_dir`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test** — add to `internal/config/config_test.go`:

```go
func TestConfig_ConfigDir_GetSet(t *testing.T) {
	c := &Config{}
	if err := c.Set(KeyConfigDir, "/x/.config"); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(KeyConfigDir)
	if err != nil || got != "/x/.config" {
		t.Fatalf("Get(config_dir) = %q, %v", got, err)
	}
}

func TestConfig_LinkRootRejected(t *testing.T) {
	_, err := loadFrom(strings.NewReader("link_root: ~/.config\n"))
	if err == nil {
		t.Fatal("expected strict-decode error for removed link_root field")
	}
}
```

Ensure `strings` is imported in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run 'TestConfig_ConfigDir_GetSet|TestConfig_LinkRootRejected'`
Expected: FAIL — `KeyConfigDir` undefined (compile error).

- [ ] **Step 3: Edit `internal/config/config.go`** — replace the const block (16-21), `Keys` (24), struct (27-32), and the `Get`/`Set` `KeyLinkRoot` cases:

Const block:
```go
const (
	KeyDotfiles     = "dotfiles"
	KeyBinDir       = "bin_dir"
	KeyGeneratedDir = "generated_dir"
	KeyConfigDir    = "config_dir"
)
```

Keys:
```go
var Keys = []string{KeyDotfiles, KeyBinDir, KeyGeneratedDir, KeyConfigDir}
```

Struct:
```go
type Config struct {
	Dotfiles     string `yaml:"dotfiles"`
	BinDir       string `yaml:"bin_dir"`
	GeneratedDir string `yaml:"generated_dir"`
	ConfigDir    string `yaml:"config_dir"`
}
```

In `Get`, replace the `case KeyLinkRoot: return c.LinkRoot, nil` with:
```go
	case KeyConfigDir:
		return c.ConfigDir, nil
```

In `Set`, replace the `case KeyLinkRoot: c.LinkRoot = value` with:
```go
	case KeyConfigDir:
		c.ConfigDir = value
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS (all of them — config tests are self-contained).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): replace link_root field with config_dir"
```

---

## Task 3: `ecosystem.Home()` accessor; remove `DefaultLinkRoot`

**Files:**
- Modify: `internal/ecosystem/ecosystem.go` (~94-98)
- Test: `internal/ecosystem/ecosystem_test.go`

- [ ] **Step 1: Write the failing test** — add to `internal/ecosystem/ecosystem_test.go`:

```go
func TestHome_RespectsHOME(t *testing.T) {
	t.Setenv("HOME", "/home/respected")
	got, err := Home()
	if err != nil || got != "/home/respected" {
		t.Fatalf("Home() = %q, %v; want /home/respected", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ecosystem/ -run TestHome_RespectsHOME`
Expected: FAIL — `Home` undefined.

- [ ] **Step 3: Replace `DefaultLinkRoot`** in `internal/ecosystem/ecosystem.go` (lines 94-98) with:

```go
// Home returns the user's home directory ($HOME on linux/darwin), the single
// canonical accessor for "~". It is NOT a resolvable knob — there is no flag,
// env, or config tier; $HOME is authoritative (the universal convention, like
// $EDITOR). Callers use this everywhere instead of os.UserHomeDir directly so
// the error message is uniform.
func Home() (string, error) {
	return userHome()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ecosystem/ -run TestHome_RespectsHOME`
Expected: PASS. (The package won't fully build until callers of `DefaultLinkRoot` are removed in Task 4 — but this package's own tests compile. If `DefaultLinkRoot` still has in-package references, there are none; cmd refs are handled next.)

- [ ] **Step 5: Commit**

```bash
git add internal/ecosystem/ecosystem.go internal/ecosystem/ecosystem_test.go
git commit -m "feat(ecosystem): add Home() accessor, remove DefaultLinkRoot"
```

---

## Task 4: cmd wiring — flags, `resolvePaths`, `buildActOptions`

**Files:**
- Modify: `cmd/dotd/main.go` (cfg struct 117-139, flags 155-168, pathFlagOwners 36-60, resolvePaths 388 + add configDir, buildActOptions 433-441, ValidateNodes call 489)
- Modify callers: `cmd/dotd/unapply_cmd.go:60`, `cmd/dotd/compose_cmd.go:98`, `cmd/dotd/main.go:518`
- Test: `cmd/dotd/main_test.go` (a focused resolve test)

- [ ] **Step 1: Write the failing test** — add to `cmd/dotd/main_test.go`:

```go
func TestResolvePaths_ConfigDirDefaultsXDG(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "/xdg/conf")
	t.Setenv("DOTD_CONFIG_FILE", filepath.Join(t.TempDir(), "nope.yaml"))
	cfg := &config{}
	if err := resolvePaths(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.configDir != "/xdg/conf" {
		t.Fatalf("configDir = %q, want /xdg/conf", cfg.configDir)
	}
}
```

Ensure `path/filepath` is imported in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotd/ -run TestResolvePaths_ConfigDirDefaultsXDG`
Expected: FAIL — `cfg.configDir` undefined (compile error).

- [ ] **Step 3: Edit the cfg struct** in `cmd/dotd/main.go` (~117-130): replace the `linkRoot string` field with `configDir string`:

```go
	initFile     string
	configDir    string
	binDir       string
	generatedDir string
```

- [ ] **Step 4: Edit the flag registrations** in `cmd/dotd/main.go` (155-163). Replace lines 157, 158, and 161 so:
  - `--config` → `--dotd-config` (rename), `--env-file` → `--dotd-env` (rename),
  - remove the `--link-root` line,
  - add `--config-dir`:

```go
	pf.StringVar(&cfg.configPath, "dotd-config", "", "path to dot-dagger's own config.yaml (default: $DOTD_CONFIG_FILE → ~/.config/dot-dagger/config.yaml)")
	pf.StringVar(&cfg.envFile, "dotd-env", "", fmt.Sprintf("path to dot-dagger's own %s (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/%s)", ecosystem.EnvFileName, ecosystem.EnvFileName))
```

and replace the old `--link-root` line (161) with:

```go
	pf.StringVar(&cfg.configDir, "config-dir", "", "config route: where $config link destinations resolve (default: config.yaml config_dir → $XDG_CONFIG_HOME)")
```

- [ ] **Step 5: Edit `pathFlagOwners`** in `cmd/dotd/main.go` (36-60): replace the entire `"link-root"` entry with a `"config-dir"` entry (same owner set, minus `dotd setup` which keeps it, plus the route consumers):

```go
	"config-dir": {
		"dotd apply": true, "dotd check": true, "dotd adopt": true,
		"dotd init": true, "dotd setup": true,
		"dotd teardown": true, "dotd unapply": true,
	},
```

- [ ] **Step 6: Edit `resolvePaths`** in `cmd/dotd/main.go`: replace the `cfg.linkRoot` resolution block (388-391) with a `configDir` resolution block:

```go
	cfg.configDir, err = ecosystem.ResolvePath(cfg.configDir, "DOTD_CONFIG_DIR", toolCfg.ConfigDir, ecosystem.XdgConfigHome)
	if err != nil {
		return err
	}
```

- [ ] **Step 7: Rewrite `buildActOptions`** in `cmd/dotd/main.go` (430-441) to read `$HOME` via the accessor and return an error:

```go
// buildActOptions constructs pipeline.ActOptions from cfg.
// dryRun forces dry-run mode regardless of cfg.dryRun. HomeDir is the real
// $HOME (via ecosystem.Home); it is never configurable.
func buildActOptions(cfg *config, dryRun bool) (pipeline.ActOptions, error) {
	home, err := ecosystem.Home()
	if err != nil {
		return pipeline.ActOptions{}, err
	}
	return pipeline.ActOptions{
		HomeDir:      home,
		BinDir:       cfg.binDir,
		ConfigDir:    cfg.configDir,
		GeneratedDir: cfg.generatedDir,
		DryRun:       dryRun || cfg.dryRun,
		Force:        cfg.force,
	}, nil
}
```

- [ ] **Step 8: Update the three `buildActOptions` callers** to handle the error:

`cmd/dotd/main.go:518`:
```go
	actOpts, err := buildActOptions(cfg, dryRun)
	if err != nil {
		return nil, 0, 0, err
	}
```
(Confirm the surrounding function returns `(..., error)` with that arity — it is the same `pipelineRun` builder that already returns `(nil, 0, 0, err)`.)

`cmd/dotd/unapply_cmd.go:60`:
```go
		actOpts, err := buildActOptions(cfg, true)
		if err != nil {
			return err
		}
```

`cmd/dotd/compose_cmd.go:98`:
```go
			actOpts, err := buildActOptions(cfg, true)
			if err != nil {
				return err
			}
```
(Adjust the `return` to match each function's signature — check the few lines around each call site.)

- [ ] **Step 9: Update the `ValidateNodes` call** at `cmd/dotd/main.go:489` to use the resolved anchors. Since `buildActOptions` now returns the full option set (DryRun is irrelevant to validation), reuse it:

```go
	valOpts, err := buildActOptions(cfg, true)
	if err != nil {
		return nil, 0, 0, err
	}
	if err := pipeline.ValidateNodes(nodes, valOpts); err != nil {
		return nil, 0, 0, err
	}
```

- [ ] **Step 10: Run the resolve test + vet**

Run: `go test ./cmd/dotd/ -run TestResolvePaths_ConfigDirDefaultsXDG && go vet ./cmd/dotd/`
Expected: PASS + vet clean for the package's compilation. Other cmd tests using `--link-root` will FAIL — fixed in Task 9.

- [ ] **Step 11: Commit**

```bash
git add cmd/dotd/main.go cmd/dotd/unapply_cmd.go cmd/dotd/compose_cmd.go cmd/dotd/main_test.go
git commit -m "feat(cmd): wire config_dir route, rename flags to --dotd-*, drop --link-root"
```

---

## Task 5: setup wizard — drop "Link root", add "Config directory"

**Files:**
- Modify: `cmd/dotd/setup_cmd.go` (home accessor 52, isUpdate 64, prompts 80-95, toolCfg 103-108)

- [ ] **Step 1: Switch the home read** at `setup_cmd.go:52` from `os.UserHomeDir()` to the accessor:

```go
	// home is used only to expand "~" in user-typed paths, not for config resolution.
	home, err := ecosystem.Home()
	if err != nil {
		return err
	}
```

- [ ] **Step 2: Update `isUpdate`** at `setup_cmd.go:64` — replace `existing.LinkRoot != ""` with `existing.ConfigDir != ""`.

- [ ] **Step 3: Replace the "Link root" prompt** (lines 95-98) with a "Config directory" prompt:

```go
	configDir, err := promptPath(out, reader, "Config directory", "Where $config link destinations resolve — your XDG config home (default: $XDG_CONFIG_HOME).", existing.ConfigDir, cfg.configDir, home, nonInteractive)
	if err != nil {
		return err
	}
```

- [ ] **Step 4: Update the written `toolCfg`** (103-108) — replace `LinkRoot: linkRoot` with `ConfigDir: configDir`:

```go
	toolCfg := &dotcfg.Config{
		Dotfiles:     dotfilesPath,
		BinDir:       binDir,
		GeneratedDir: generatedDir,
		ConfigDir:    configDir,
	}
```

- [ ] **Step 5: Build + run setup tests**

Run: `go test ./cmd/dotd/ -run 'Setup'`
Expected: setup tests that pipe prompt lines may need a line added/removed (the "Link root" prompt became "Config directory" — net zero prompt count, just a label/semantics change). Update any test that asserts the prompt label or the written `link_root:`/`config_dir:` value. If a setup test feeds N input lines, it still needs N (one prompt swapped, not added).

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/setup_cmd.go cmd/dotd/*_test.go
git commit -m "feat(setup): replace Link root prompt with Config directory"
```

---

## Task 6: init — scaffold tokens + source-line home

**Files:**
- Modify: `cmd/dotd/init_cmd.go` (conventionRoles 145-164, DetectShellConfig 102, AppendSourceLine 131)

- [ ] **Step 1: Update the source-line home reads.** In `maybeAddSourceLine`, compute the real home once and pass it instead of `cfg.linkRoot`. After line 101 (`shell == ""` guard) add:

```go
	home, err := ecosystem.Home()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
```

Then change line 102 `cfg.linkRoot` → `home`, and line 131 `cfg.linkRoot` → `home`. (Rename the existing `err` handling if a redeclaration conflict arises — the function already uses `err` for `resolveEnv`; reuse the variable with `=` not `:=` if needed, or scope the home read before the first `err` use.)

- [ ] **Step 2: Update the config-dir scaffold role** (`conventionRoles`, the "Config files" entry, 152-157):

```go
	{
		label:   "Config files",
		desc:    "Files here are symlinked into your config dir by default (e.g. config/nvim/init.lua → $config/nvim/init.lua, i.e. $XDG_CONFIG_HOME).",
		defDir:  adopter.DirConfig,
		content: "link_root: \"" + pipeline.ConfigPrefix + "\"\ndefaults:\n  actions:\n    - link\n",
	},
```

(The "Bin scripts" entry already uses `pipeline.BinPrefix`, whose value is now `$bin` — no edit needed there, but verify it renders `link_root: "$bin"`.)

- [ ] **Step 3: Build + run init tests**

Run: `go test ./cmd/dotd/ -run 'Init|Scaffold'`
Expected: tests asserting scaffolded `.dagger` content must expect `link_root: "$config"` / `link_root: "$bin"`. Update those assertions. Should PASS after.

- [ ] **Step 4: Commit**

```bash
git add cmd/dotd/init_cmd.go cmd/dotd/*_test.go
git commit -m "feat(init): scaffold \$config/\$bin tokens, use ecosystem.Home for source line"
```

---

## Task 7: teardown + adopt home reads

**Files:**
- Modify: `cmd/dotd/teardown_cmd.go:74`
- Modify: `cmd/dotd/adopt.go:119-125`
- Modify: `internal/adopter/adopter.go` (`AdoptOptions` 47-48, ActOptions build 120-125)

- [ ] **Step 1: teardown** — in `teardown_cmd.go`, before the loop that uses `cfg.linkRoot` (line 74), compute home and use it:

```go
	home, err := ecosystem.Home()
	if err != nil {
		return err
	}
```
Then change line 74 `cfg.linkRoot` → `home`. (Place the home read where `err` is already in scope or declare appropriately; confirm the enclosing function returns `error`.)

- [ ] **Step 2: adopter struct** — in `internal/adopter/adopter.go`, rename the `LinkRoot` field to `HomeDir` and add `ConfigDir` (47-48):

```go
	HomeDir      string // resolved real $HOME (for "~" expansion)
	ConfigDir    string // resolved config route (for "$config" expansion)
	BinDir       string // resolved managed bin dir
```

- [ ] **Step 3: adopter ActOptions** — update the build at `adopter.go:120-125`:

```go
	actOpts := pipeline.ActOptions{
		HomeDir:   opts.HomeDir,
		BinDir:    opts.BinDir,
		ConfigDir: opts.ConfigDir,
		DryRun:    opts.DryRun,
		Force:     opts.Force,
	}
```

- [ ] **Step 4: adopt cmd** — in `cmd/dotd/adopt.go`, compute home and populate the renamed/new fields (119-125):

```go
	home, err := ecosystem.Home()
	if err != nil {
		return err
	}
	opts := adopter.AdoptOptions{
		DotfilesRoot: cfg.files,
		Conventions:  conv,
		HomeDir:      home,
		ConfigDir:    cfg.configDir,
		BinDir:       cfg.binDir,
		Force:        cfg.force,
	}
```
(If `err` is already declared earlier in `runAdopt`, use `=`; verify the surrounding code.)

- [ ] **Step 5: Build + run adopter/teardown tests**

Run: `go test ./internal/adopter/ ./cmd/dotd/ -run 'Adopt|Teardown'`
Expected: adopter tests that set `LinkRoot:` in `AdoptOptions` must be renamed to `HomeDir:`. Update + PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/teardown_cmd.go cmd/dotd/adopt.go internal/adopter/adopter.go internal/adopter/*_test.go
git commit -m "feat(adopt,teardown): use ecosystem.Home, add ConfigDir to adopt"
```

---

## Task 8: config subcommand help examples

**Files:**
- Modify: `cmd/dotd/config_cmd.go:96,121`

- [ ] **Step 1: Update the help examples** — replace `link_root` with `config_dir`:
  - Line 96: `dotd config get link_root` → `dotd config get config_dir`
  - Line 121: `dotd config set link_root /home/me` → `dotd config set config_dir ~/.config`

- [ ] **Step 2: Build**

Run: `go build ./cmd/dotd/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add cmd/dotd/config_cmd.go
git commit -m "docs(config): update help examples link_root -> config_dir"
```

---

## Task 9: fixtures + tests — `~bin`→`$bin`, `--link-root`→env config

**Files:** testdata `.dagger` fixtures + Go tests + e2e scripts. **This is the task that makes the full suite green.**

- [ ] **Step 1: Enumerate the blast radius first (audit lesson).** Run and read the output before editing:

```bash
grep -rn '~bin\|~/\.config\|--link-root\|link-root\|LinkRoot\|DOTD_LINK_ROOT\|link_root' \
  internal/ cmd/ test/ --include='*.go' --include='*.dagger' --include='*.sh' --include='*.yaml'
```
Expected: a finite list. Triage each hit:
  - **per-node `link_root:` in a `.dagger`** with value `~` or an abs path → leave (that key stays).
  - value `~bin` → change to `$bin`.
  - value `~/.config` in a *test fixture that exercises the config route* → change to `$config`; leave literal `~/.config` only where the test specifically asserts `$HOME/.config` semantics.
  - `--link-root <dir>` in a Go test → delete the flag arg, add `t.Setenv("HOME", <dir>)` to that test.
  - `--link-root <dir>` in an e2e `.sh` → delete the flag arg, add `export HOME=<dir>` near the top of the script; where the test exercises config linking add `export DOTD_CONFIG_DIR=<dir>/.config` (or pass `--config-dir`).
  - `DOTD_LINK_ROOT` / `config.LinkRoot` references → remove.

- [ ] **Step 2: Update `.dagger` fixtures** using `$bin`:
  - `internal/pipeline/testdata/dotfiles/conf/.dagger` (`link_root: "~"` → leave; it's `~`)
  - `test/e2e/fixture/bin/.dagger` (`~bin` → `$bin`)
  - `cmd/dotd/testdata/dotfiles/bin/.dagger` (`~bin` → `$bin`)
  - `cmd/dotd/testdata/dotfiles/config/.dagger` (`~/.config` → `$config`)
  Use exact edits per the grep output (there may be more — trust the grep, not this list).

- [ ] **Step 3: Convert Go tests** from `--link-root` to `t.Setenv("HOME", ...)`. For each hit in `*_test.go`, remove the flag from the args slice and add the env set. Example transform:

```go
// before: args := []string{"apply", "--link-root", tmp, ...}
// after:
t.Setenv("HOME", tmp)
args := []string{"apply", ...}
```

- [ ] **Step 4: Convert e2e scripts.** For each `test/e2e/*.sh` hit, replace `--link-root "$X"` usages with `export HOME="$X"` (set once, near where the fake home is established), plus `export DOTD_CONFIG_DIR` where a config-route assertion exists.

- [ ] **Step 5: Run the full Go suite**

Run: `go test ./... && go vet ./...`
Expected: PASS, vet clean. Iterate on any remaining `~bin`/`link-root` stragglers until green.

- [ ] **Step 6: Run gofmt**

Run: `gofmt -l internal/ cmd/` (expect no output) — if any file is listed, `gofmt -w` it.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test: convert fixtures to \$bin/\$config and tests to HOME env config"
```

---

## Task 10: docs

**Files:** `docs/reference/dotd.md`, concepts docs, `.claude/docs/spec/symlinks.md`, `.claude/docs/spec/cli.md`, `.claude/docs/spec/env.md`

- [ ] **Step 1: Find doc references**

```bash
grep -rn 'link_root\|link-root\|~bin\|--config \|--env-file\|~/\.config' docs/ .claude/docs/spec/
```

- [ ] **Step 2: Update each hit** to the new model:
  - tokens: `~` (=$HOME), `$bin`, `$config`
  - flags: `--dotd-config`, `--dotd-env`, `--config-dir`; remove `--link-root`
  - config keys: `config_dir` replaces `link_root`
  - Explain `~`=$HOME always (never configurable); `$config` default `$XDG_CONFIG_HOME`.

- [ ] **Step 3: Commit**

```bash
git add docs/ .claude/docs/spec/
git commit -m "docs: roots model — tokens, --dotd-* flags, config_dir"
```

---

## Task 11: final validation + tracker cleanup

- [ ] **Step 1: Full verification**

```bash
go test ./... && go vet ./... && gofmt -l internal/ cmd/
```
Expected: all pass, no gofmt output.

- [ ] **Step 2: Smoke-test the CLI**

```bash
go run ./cmd/dotd help | grep -E 'dotd-config|config-dir' && \
go run ./cmd/dotd --help 2>&1 | grep -v 'link-root'
```
Expected: new flags present, no `link-root`.

- [ ] **Step 3: Update `.claude/TODO.md`** — mark the 🔴 TOP PRIORITY section DONE (move to a completed note), and update the memory file `project_link_root_overhaul.md` to "shipped".

- [ ] **Step 4: Open the PR** (per CLAUDE.md, confirm branch not merged first):

```bash
gh pr create --title "feat: roots model — kill global link_root, add \$config route" \
  --body "Implements docs/superpowers/specs/2026-06-13-roots-model-design.md"
```

---

## Self-Review notes

- **Spec coverage:** three-anchor model (T1), `config_dir` knob + resolution (T2,T4), flag namespace `--dotd-*`/`--config-dir` (T4), `ecosystem.Home()` no-tracking (T3,T5,T6,T7), expansion semantics (T1), `$HOME` consumers all switched (T4,T5,T6,T7), migration: drop field strict-decode (T2), rename flags no alias (T4), tests→env (T9), scaffold `$config` (T6). Out-of-scope items (per-node key, `bin_dir` behavior, `generated_dir`) untouched. ✓
- **Type consistency:** `ActOptions.ConfigDir`, `ConfigPrefix="$config"`, `BinPrefix="$bin"`, `KeyConfigDir="config_dir"`, `Config.ConfigDir`, `AdoptOptions.HomeDir`/`.ConfigDir`, `ecosystem.Home()`, `buildActOptions(...) (ActOptions, error)` used consistently across tasks. ✓
- **No placeholders:** every code step shows real code; test-update steps point at the grep output as the authoritative site list (the one place exhaustive enumeration must happen at execution time, by design — fixtures/tests can drift).
