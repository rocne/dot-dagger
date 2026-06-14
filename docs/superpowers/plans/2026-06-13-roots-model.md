# Roots Model (pure-XDG) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the global `link_root` knob with three knob-less anchor tokens (`~`, `$bin`, `$config`) resolved purely from the environment (XDG), remove every path-route/output knob, add anchor-token validation and a `dotd paths` view, and rename the tool's own state-file flags to `--dotd-config`/`--dotd-env`.

**Architecture:** `.dagger` `link_root:`/dest values use anchor tokens; the pipeline stays a pure function receiving resolved `HomeDir`/`BinDir`/`ConfigDir`/`GeneratedDir` in `ActOptions`. All anchors + tool outputs are resolved once in `resolvePaths()` from `ecosystem` accessors (no flag/env/config-field tiers) into `cfg` fields. config.yaml shrinks to a single `dotfiles` field. No backward compatibility — tests configure `HOME`/`XDG_*`.

**Tech Stack:** Go 1.26, cobra/pflag, yaml.v3 (strict `KnownFields`).

**Spec:** `docs/superpowers/specs/2026-06-13-roots-model-design.md`

**Compile-boundary note (important):** Go compiles per *package*, not per file. The
`cmd/dotd` changes are mutually dependent (config-field removal + `cfg` rename +
every reference + setup/adopter) and only compile as a unit — so they live in a
single task (Task 3) with one build/test/commit boundary. Task 1 also flips
`BinPrefix` (`~bin`→`$bin`), so `internal/pipeline` and the whole repo only go
fully green after the fixtures task (Task 5). Each task states exactly what it
must make pass.

---

## File Structure

| File | Change | Task |
|------|--------|------|
| `internal/pipeline/act.go` | `$bin`/`$config`/`~` `expandDest`; `BinPrefix="$bin"`, `ConfigPrefix="$config"`; `ActOptions.ConfigDir` | 1 |
| `internal/pipeline/actions.go` | thread `ConfigDir`; `validateAnchor` + wire into `validateNode` | 1 |
| `internal/ecosystem/ecosystem.go` (+ test) | `Home`/`XdgBinHome`/`BinDir`/`ConfigDir`; rename `DefaultGeneratedDir`→`GeneratedDir`, `DefaultInitFile`→`InitFile`; remove `DefaultLinkRoot`/`DefaultBinDir`; update existing tests | 2 |
| `internal/config/config.go` | strip to `Dotfiles` only | 3 |
| `cmd/dotd/main.go` | cfg fields; drop path flags; rename `--dotd-*`; `resolvePaths` from accessors; `pathFlagOwners`; `buildActOptions`; `ValidateNodes` | 3 |
| `cmd/dotd/setup_cmd.go` | strip to `dotfiles` prompt; `Home()` | 3 |
| `cmd/dotd/init_cmd.go` | scaffold `$bin`/`$config`; `cfg.home` | 3 |
| `cmd/dotd/teardown_cmd.go`, `adopt.go`, `internal/adopter/adopter.go` | `cfg.home`; adopt `ConfigDir` | 3 |
| `cmd/dotd/config_cmd.go` | help examples → `dotfiles` | 3 |
| `cmd/dotd/paths_cmd.go` (new) | `dotd paths` resolved-view | 4 |
| testdata/e2e/docs | `~bin`→`$bin`; `--link-root`→env; doc updates | 5, 6 |

---

## Task 1: Pipeline token expansion + validation

**Files:** Modify `internal/pipeline/act.go`, `internal/pipeline/actions.go`; Test `internal/pipeline/act_test.go`, `internal/pipeline/actions_test.go`

- [ ] **Step 1: Write the failing expansion test** — add to `internal/pipeline/act_test.go`:

```go
func TestExpandDest_Anchors(t *testing.T) {
	const home, bin, conf = "/home/u", "/home/u/.local/bin/dot-dagger", "/home/u/.config"
	cases := []struct{ in, want string }{
		{"~", home}, {"~/.zshrc", home + "/.zshrc"},
		{"$bin", bin}, {"$bin/fmt", bin + "/fmt"},
		{"$config", conf}, {"$config/nvim/init.lua", conf + "/nvim/init.lua"},
		{"/abs/path", "/abs/path"}, {"relative/path", "relative/path"},
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
Expected: FAIL — `expandDest` currently takes 3 args (compile error).

- [ ] **Step 3: Update `ActOptions` + token consts** in `internal/pipeline/act.go`. Replace the `ActOptions` struct and `BinPrefix` const:

```go
// ActOptions configures Act behaviour.
type ActOptions struct {
	HomeDir      string // replaces "~" in link destinations; the real $HOME
	BinDir       string // replaces "$bin" in link destinations
	ConfigDir    string // replaces "$config" in link destinations
	GeneratedDir string // directory for compose-generated files
	DryRun       bool   // validate without writing to filesystem
	Force        bool   // overwrite non-symlink files on link conflicts
}

// BinPrefix and ConfigPrefix are the destination tokens that expand to
// ActOptions.BinDir / ActOptions.ConfigDir ("$bin", "$config", optionally
// "/<rel>"). "~" expands to HomeDir (the real $HOME) and is never configurable.
const (
	BinPrefix    = "$bin"
	ConfigPrefix = "$config"
)
```

- [ ] **Step 4: Rewrite `expandDest`** in `internal/pipeline/act.go`:

```go
// expandDest expands the "~", "$bin", and "$config" anchor tokens in a link
// destination. "~" / "~/x" use the real $HOME; "$bin" / "$config" (optionally
// "/<rel>") map to binDir / configDir. Absolute and relative paths are returned
// unchanged.
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
// path is exactly token or token+"/...". Returns "" when path does not match or
// base is empty.
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

- [ ] **Step 5: Thread `configDir` through `resolveLink`** in `act.go`:

```go
func resolveLink(dest string, n RawNode, homeDir, binDir, configDir string) string {
	if dest == "" {
		dest = deriveLinkDest(n)
	}
	return expandDest(dest, homeDir, binDir, configDir)
}
```

Update its caller in `act.go` (`ActionLink` case in `emitNodeActions`):
```go
			dest := resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```
And in `actions.go` (`CheckLinkConflicts`):
```go
				dest = resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```

- [ ] **Step 6: Fix the `Act` HomeDir error message** in `act.go`:
```go
		return nil, fmt.Errorf("act: HomeDir is required — set it to the resolved $HOME")
```

- [ ] **Step 7: Run the expansion test**

Run: `go test ./internal/pipeline/ -run TestExpandDest_Anchors`
Expected: PASS. (Whole-package tests with `~bin` fixtures still fail — fixed in Task 5.)

- [ ] **Step 8: Commit**

```bash
git add internal/pipeline/act.go internal/pipeline/actions.go internal/pipeline/act_test.go
git commit -m "feat(pipeline): \$bin/\$config/~ anchor expansion"
```

- [ ] **Step 9: Write the failing validation test** — add to `internal/pipeline/actions_test.go` (create with `package pipeline` if absent):

```go
func TestValidateAnchor(t *testing.T) {
	ok := []string{"", "~", "~/.zshrc", "$bin", "$bin/fmt", "$config", "$config/nvim", "/abs", "rel/path"}
	for _, v := range ok {
		if err := validateAnchor("link_root", v); err != nil {
			t.Errorf("validateAnchor(%q) = %v, want nil", v, err)
		}
	}
	bad := []string{"~bin", "~config", "$conifg", "$HOME", "$binary", "~root/x"}
	for _, v := range bad {
		if err := validateAnchor("link_root", v); err == nil {
			t.Errorf("validateAnchor(%q) = nil, want error", v)
		}
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/pipeline/ -run TestValidateAnchor`
Expected: FAIL — `validateAnchor` undefined.

- [ ] **Step 11: Implement `validateAnchor` + wire into `validateNode`** in `internal/pipeline/actions.go`.

Add (near `validateNode`):
```go
// validateAnchor rejects a link destination or link_root value that begins with
// an anchor sigil ("~" or "$") but is not a recognized token. Catches typos like
// "$conifg" that expandDest would otherwise treat as a literal path. Paths with
// no leading sigil are always allowed. field labels the error source.
func validateAnchor(field, value string) error {
	if value == "" {
		return nil
	}
	switch value[0] {
	case '~':
		if value == "~" || strings.HasPrefix(value, "~/") {
			return nil
		}
	case '$':
		for _, tok := range []string{BinPrefix, ConfigPrefix} {
			if value == tok || strings.HasPrefix(value, tok+"/") {
				return nil
			}
		}
	default:
		return nil
	}
	return fmt.Errorf("unknown anchor token %q in %s — valid anchors are ~, %s, %s", value, field, BinPrefix, ConfigPrefix)
}
```

In `validateNode`, after the `if n.IsCompose { return nil }` guard:
```go
	if err := validateAnchor("link_root", n.LinkRoot); err != nil {
		return fmt.Errorf("node %s: %w", n.LogicalName, err)
	}
```
Inside the `case ActionLink:` block (after the empty-dest check):
```go
			if err := validateAnchor("link destination", a.Dest); err != nil {
				return fmt.Errorf("node %s: %w", n.LogicalName, err)
			}
```
(`strings` and `fmt` are already imported in `actions.go`.)

- [ ] **Step 12: Run the validation test**

Run: `go test ./internal/pipeline/ -run 'TestValidateAnchor|TestExpandDest'`
Expected: PASS.

- [ ] **Step 13: Commit**

```bash
git add internal/pipeline/actions.go internal/pipeline/actions_test.go
git commit -m "feat(pipeline): reject unknown anchor tokens"
```

---

## Task 2: ecosystem accessors (+ update existing tests)

**Files:** Modify `internal/ecosystem/ecosystem.go`, `internal/ecosystem/ecosystem_test.go`

> P1 note: the existing test file references `DefaultLinkRoot`/`DefaultBinDir`/`DefaultGeneratedDir`/`DefaultInitFile` (lines ~63–287). These MUST be updated in this task or the package won't compile.

- [ ] **Step 1: Write the new accessor tests** — replace `TestDefaultLinkRootUsesHOME` and `TestDefaultBinDir` in `internal/ecosystem/ecosystem_test.go` with:

```go
func TestHome_RespectsHOME(t *testing.T) {
	t.Setenv("HOME", "/home/respected")
	if got, err := ecosystem.Home(); err != nil || got != "/home/respected" {
		t.Fatalf("Home() = %q, %v; want /home/respected", got, err)
	}
}

func TestBinDir_NamespacedHonorsXDG(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_BIN_HOME", "")
	if got, _ := ecosystem.BinDir(); got != "/home/u/.local/bin/"+ecosystem.Name {
		t.Fatalf("BinDir() default = %q", got)
	}
	t.Setenv("XDG_BIN_HOME", "/custom/bin")
	if got, _ := ecosystem.BinDir(); got != "/custom/bin/"+ecosystem.Name {
		t.Fatalf("BinDir() with XDG_BIN_HOME = %q", got)
	}
}
```

Also update the **error-without-HOME subtests** (lines ~256–287): rename `DefaultBinDir`→`BinDir`, `DefaultLinkRoot`→`Home`, `DefaultGeneratedDir`→`GeneratedDir`, `DefaultInitFile`→`InitFile`. And rename the calls in `TestDefaultInitFileUsesXDGDataHome`/`TestDefaultGeneratedDir` to `InitFile()`/`GeneratedDir()` (rename the test funcs too for clarity: `TestInitFile…`/`TestGeneratedDir`).

- [ ] **Step 2: Run tests to verify they fail to compile**

Run: `go test ./internal/ecosystem/ -run 'TestHome|TestBinDir'`
Expected: FAIL — `Home`, `BinDir` undefined (compile error).

- [ ] **Step 3: Replace `DefaultLinkRoot` and `DefaultBinDir`** in `internal/ecosystem/ecosystem.go`. Remove both; add:

```go
// Home returns the user's home directory ($HOME on linux/darwin) — the single
// canonical accessor for "~". Not a configurable knob: $HOME is authoritative
// (universal convention, like $EDITOR).
func Home() (string, error) {
	return userHome()
}

// XdgBinHome returns $XDG_BIN_HOME if set to an absolute path, else ~/.local/bin.
// $XDG_BIN_HOME is not in the XDG base spec but is the de-facto convention for
// user binaries; honoring it lets users relocate the bin root the standard way.
func XdgBinHome() (string, error) {
	if d := os.Getenv("XDG_BIN_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// BinDir returns the dot-dagger-namespaced bin route: <XdgBinHome>/dot-dagger.
// Namespacing is free because PATH is a search list; init.sh adds it to PATH.
func BinDir() (string, error) {
	base, err := XdgBinHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Name), nil
}

// ConfigDir returns the config route: $XDG_CONFIG_HOME (≈ ~/.config). Configs
// link directly here (apps read ~/.config/<app>), never namespaced.
func ConfigDir() (string, error) {
	return xdgConfigHome()
}
```

- [ ] **Step 4: Rename `DefaultGeneratedDir`→`GeneratedDir`, `DefaultInitFile`→`InitFile`** (keep bodies; drop "Default" from name + doc comment). Leave `DefaultConfigFile`/`DefaultEnvFile`/`DefaultDotfiles` untouched (still used by surviving configurable paths).

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/ecosystem/`
Expected: PASS (whole package — all renamed/replaced tests compile and pass).

- [ ] **Step 6: Commit**

```bash
git add internal/ecosystem/ecosystem.go internal/ecosystem/ecosystem_test.go
git commit -m "feat(ecosystem): Home/XdgBinHome/BinDir/ConfigDir accessors"
```

---

## Task 3: `cmd/dotd` cutover (single compile-unit)

> **This is one atomic change.** config.yaml strip + `cfg` field rename + every reference + flag changes + setup/init/teardown/adopt/adopter only compile together. Do every step, then ONE build/test/commit at the end. `go test ./cmd/dotd/` will not compile until Step 12.

**Files:** `internal/config/config.go`, `cmd/dotd/main.go`, `setup_cmd.go`, `init_cmd.go`, `teardown_cmd.go`, `adopt.go`, `config_cmd.go`, `internal/adopter/adopter.go`; Tests `internal/config/config_test.go`, `cmd/dotd/main_test.go`

- [ ] **Step 1: Strip `internal/config/config.go` to `Dotfiles` only.** Replace the const block, `Keys`, struct, and `Get`/`Set` bodies:

```go
const KeyDotfiles = "dotfiles"

// Keys is the ordered list of all valid config keys.
var Keys = []string{KeyDotfiles}
```
```go
type Config struct {
	Dotfiles string `yaml:"dotfiles"`
}
```
```go
func (c *Config) Get(key string) (string, error) {
	switch key {
	case KeyDotfiles:
		return c.Dotfiles, nil
	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

func (c *Config) Set(key, value string) error {
	switch key {
	case KeyDotfiles:
		c.Dotfiles = value
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	return nil
}
```
No special-case handling for removed fields — strict `KnownFields(true)` rejecting them is the desired behavior.

- [ ] **Step 2: Add the config test** to `internal/config/config_test.go` (import `strings`):

```go
func TestConfig_OnlyDotfiles(t *testing.T) {
	if len(Keys) != 1 || Keys[0] != KeyDotfiles {
		t.Fatalf("Keys = %v, want [dotfiles]", Keys)
	}
	for _, field := range []string{"bin_dir: /x\n", "link_root: ~/.config\n", "generated_dir: /g\n"} {
		if _, err := loadFrom(strings.NewReader(field)); err == nil {
			t.Errorf("expected strict-decode error for removed field: %q", field)
		}
	}
}
```

Run now (config package compiles independently): `go test ./internal/config/` → PASS. (Do NOT commit yet — cmd/dotd is about to break.)

- [ ] **Step 3: `cmd/dotd/main.go` — cfg struct.** Replace `linkRoot string` with `home string`; add `configDir string`; keep `binDir`/`generatedDir`/`initFile`/`envFile`/`configPath`/`files`.

- [ ] **Step 4: `main.go` — flags.** Rename `--config`→`--dotd-config`, `--env-file`→`--dotd-env`; **delete** the `--init-file`, `--link-root`, `--bin-dir`, `--generated-dir` registrations:

```go
	pf.StringVar(&cfg.configPath, "dotd-config", "", "path to dot-dagger's own config.yaml (default: $DOTD_CONFIG_FILE → ~/.config/dot-dagger/config.yaml)")
	pf.StringVar(&cfg.envFile, "dotd-env", "", fmt.Sprintf("path to dot-dagger's own %s (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/%s)", ecosystem.EnvFileName, ecosystem.EnvFileName))
```

- [ ] **Step 5: `main.go` — `pathFlagOwners`** shrinks to behavior flags only:

```go
var pathFlagOwners = map[string]map[string]bool{
	"dry-run": {
		"dotd apply": true, "dotd adopt": true,
		"dotd unapply": true, "dotd teardown": true,
	},
	"force": {
		"dotd apply": true, "dotd adopt": true,
	},
}
```

- [ ] **Step 6: `main.go` — `resolvePaths`.** Replace the `cfg.initFile`/`cfg.linkRoot`/`cfg.binDir`/`cfg.generatedDir` `ResolvePath` blocks with direct accessor calls; add `home`/`configDir`:

```go
	if cfg.home, err = ecosystem.Home(); err != nil {
		return err
	}
	if cfg.binDir, err = ecosystem.BinDir(); err != nil {
		return err
	}
	if cfg.configDir, err = ecosystem.ConfigDir(); err != nil {
		return err
	}
	if cfg.generatedDir, err = ecosystem.GeneratedDir(); err != nil {
		return err
	}
	if cfg.initFile, err = ecosystem.InitFile(); err != nil {
		return err
	}
```
Leave `cfg.envFile`/`cfg.configPath`/`cfg.files` resolutions intact. `toolCfg` now only has `Dotfiles` — the `filesFromCwd` line and `cfg.files` ResolvePath referencing `toolCfg.Dotfiles` stay valid; remove any `toolCfg.LinkRoot/BinDir/GeneratedDir` references.

- [ ] **Step 7: `main.go` — `buildActOptions`** (no error return; fields pre-resolved):

```go
func buildActOptions(cfg *config, dryRun bool) pipeline.ActOptions {
	return pipeline.ActOptions{
		HomeDir:      cfg.home,
		BinDir:       cfg.binDir,
		ConfigDir:    cfg.configDir,
		GeneratedDir: cfg.generatedDir,
		DryRun:       dryRun || cfg.dryRun,
		Force:        cfg.force,
	}
}
```
And the `ValidateNodes` call:
```go
	if err := pipeline.ValidateNodes(nodes, pipeline.ActOptions{HomeDir: cfg.home, BinDir: cfg.binDir, ConfigDir: cfg.configDir}); err != nil {
		return nil, 0, 0, err
	}
```

- [ ] **Step 8: `setup_cmd.go`.** Switch `home` to `ecosystem.Home()`; `isUpdate := existing.Dotfiles != ""`; delete the "Bin directory", "Generated files directory", "Link root" `promptPath` blocks (keep `dotfilesPath`); write `toolCfg := &dotcfg.Config{Dotfiles: dotfilesPath}`.

- [ ] **Step 9: `init_cmd.go`.** In `maybeAddSourceLine`, after the `shell == ""` guard add `home, err := ecosystem.Home()` (handle err: `return fmt.Errorf("init: %w", err)`); change `DetectShellConfig(shell, resolved["os"], cfg.linkRoot)` and `AppendSourceLine(sc.RCFile, cfg.initFile, cfg.linkRoot)` to use `home`. In `conventionRoles`, the "Config files" entry `content` → `"link_root: \"" + pipeline.ConfigPrefix + "\"\ndefaults:\n  actions:\n    - link\n"` and update its `desc` to mention `$config`. (Bin entry already uses `pipeline.BinPrefix` = `$bin`.)

- [ ] **Step 10: `teardown_cmd.go` + `adopt.go` + `internal/adopter/adopter.go`.**
  - teardown: before the `DetectShellConfig(shell, osName, cfg.linkRoot)` call add `home, err := ecosystem.Home()` (return err) and pass `home`.
  - `internal/adopter/adopter.go`: rename `AdoptOptions.LinkRoot`→`HomeDir`; add `ConfigDir string`; in the `pipeline.ActOptions` build set `HomeDir: opts.HomeDir`, `ConfigDir: opts.ConfigDir`, `BinDir: opts.BinDir`.
  - `adopt.go`: add `home, err := ecosystem.Home()` (return err); set `AdoptOptions{… HomeDir: home, ConfigDir: cfg.configDir, BinDir: cfg.binDir, …}` (drop `LinkRoot:`).

- [ ] **Step 11: `config_cmd.go` help examples** — `dotd config get link_root` → `dotd config get dotfiles`; `dotd config set link_root /home/me` → `dotd config set dotfiles ~/dotfiles`.

- [ ] **Step 12: Update `cmd/dotd/main_test.go`** for the cutover: rename `TestAdopt_DefaultLinkRoot` body to drop `--link-root` (use `t.Setenv("HOME", …)`); add the resolve test:

```go
func TestResolvePaths_AnchorsFromEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "/xdg/conf")
	t.Setenv("XDG_BIN_HOME", "/xdg/bin")
	t.Setenv("DOTD_CONFIG_FILE", filepath.Join(t.TempDir(), "nope.yaml"))
	cfg := &config{}
	if err := resolvePaths(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.home != home || cfg.configDir != "/xdg/conf" || cfg.binDir != "/xdg/bin/dot-dagger" {
		t.Fatalf("home=%q configDir=%q binDir=%q", cfg.home, cfg.configDir, cfg.binDir)
	}
}
```
(Other `cmd/dotd` tests using `--link-root`/`--bin-dir`/etc. are swept in Task 5; for now fix only what blocks compilation of `main_test.go` — i.e. references to removed flags/fields in this file. Tests in other `_test.go` files of the same package must also compile, so grep this package for `linkRoot`/`--link-root`/`existing.BinDir` and fix compile-breakers; behavior assertions can wait for Task 5 but the package must BUILD.)

- [ ] **Step 13: Build the whole repo + run cmd tests**

Run: `go build ./... && go test ./cmd/dotd/ -run 'TestResolvePaths_AnchorsFromEnv'`
Expected: build succeeds; resolve test PASSES. (Full `go test ./cmd/dotd/` may still fail on `~bin`/flag-based behavior tests — Task 5. If a *compile* error remains, fix it before committing.)

- [ ] **Step 14: Commit**

```bash
git add internal/config/ cmd/dotd/ internal/adopter/
git commit -m "feat(cmd): pure-XDG cutover — drop path knobs, config.yaml→dotfiles, --dotd-* flags"
```

---

## Task 4: `dotd paths` resolved-view

**Files:** Create `cmd/dotd/paths_cmd.go`; Modify `cmd/dotd/main.go` (register); Test `cmd/dotd/paths_cmd_test.go`

- [ ] **Step 1: Write the failing test** — `cmd/dotd/paths_cmd_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPathsCmd_ShowsResolvedAnchors(t *testing.T) {
	cfg := &config{
		home: "/h", binDir: "/h/.local/bin/dot-dagger", configDir: "/h/.config",
		generatedDir: "/h/gen", initFile: "/h/init.sh", files: "/h/dotfiles",
		configPath: "/h/.config/dot-dagger/config.yaml", envFile: "/h/.config/dot-dagger/env.yaml",
	}
	var out bytes.Buffer
	cmd := newPathsCmd(cfg)
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"$bin", "$config", "home", "init.sh", "dotfiles", "config.yaml", "env.yaml", "/h"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("paths output missing %q:\n%s", want, out.String())
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotd/ -run TestPathsCmd_ShowsResolvedAnchors`
Expected: FAIL — `newPathsCmd` undefined.

- [ ] **Step 3: Create `cmd/dotd/paths_cmd.go`** (single import block):

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type pathRow struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func newPathsCmd(cfg *config) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Show where anchors and tool paths resolve on this machine",
		Long: `Print the resolved locations of every anchor token and tool-managed path.

Examples:
  dotd paths
  dotd paths --json | jq '.[] | select(.name=="$config")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows := []pathRow{
				{"home", cfg.home},
				{"$bin", cfg.binDir},
				{"$config", cfg.configDir},
				{"generated", cfg.generatedDir},
				{"init.sh", cfg.initFile},
				{"dotfiles", cfg.files},
				{"config.yaml", cfg.configPath},
				{"env.yaml", cfg.envFile},
			}
			if jsonOutput {
				return writePathsJSON(cmd.OutOrStdout(), rows)
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-11s %s\n", r.Name, r.Path)
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}

func writePathsJSON(w io.Writer, rows []pathRow) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
```

- [ ] **Step 4: Register** in `cmd/dotd/main.go` alongside the other `root.AddCommand(...)` calls:
```go
	root.AddCommand(newPathsCmd(cfg))
```
Match the surrounding `GroupID`/group-assignment pattern if commands are grouped.

- [ ] **Step 5: Run the test + build**

Run: `go test ./cmd/dotd/ -run TestPathsCmd_ShowsResolvedAnchors && go build ./cmd/dotd/`
Expected: PASS + build OK.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/paths_cmd.go cmd/dotd/paths_cmd_test.go cmd/dotd/main.go
git commit -m "feat(cmd): add 'dotd paths' resolved-view"
```

---

## Task 5: fixtures + tests — `~bin`→`$bin`, flags→env (greens the suite)

**Files:** testdata `.dagger` fixtures + Go tests + e2e scripts

- [ ] **Step 1: Enumerate the blast radius first.** Run and read before editing:

```bash
grep -rn '~bin\|~/\.config\|--link-root\|link-root\|LinkRoot\|DOTD_LINK_ROOT\|link_root\|--bin-dir\|bin-dir\|--init-file\|init-file\|--generated-dir\|generated-dir\|DOTD_BIN_DIR\|DOTD_INIT_FILE\|DOTD_GENERATED_DIR' \
  internal/ cmd/ test/ --include='*.go' --include='*.dagger' --include='*.sh' --include='*.yaml'
```
Triage each hit:
  - per-node `.dagger` `link_root:` value `~` or abs path → leave.
  - value `~bin` → `$bin`; config-route `~/.config` in a fixture exercising config linking → `$config`.
  - **⚠ Never rewrite the tool's OWN paths.** `~/.config/dot-dagger/...` (config.yaml/env.yaml locations — `cfg.configPath`, `cfg.envFile`, `DefaultConfigFile`/`DefaultEnvFile` comments/tests) are unrelated to the `$config` route. Leave every `~/.config/dot-dagger` literal as-is. Only the dotfiles-side config *destination* (`config/.dagger` link_root) becomes `$config`.
  - `--link-root <d>`/`--bin-dir <d>`/`--init-file <f>`/`--generated-dir <d>` in a Go test → remove the flag arg; add `t.Setenv("HOME", d)` and/or `t.Setenv("XDG_BIN_HOME"/"XDG_DATA_HOME"/"XDG_CONFIG_HOME", …)` as needed.
  - same flags in e2e `.sh` → remove; `export HOME=…` (+ `XDG_*`) near the top.
  - `DOTD_LINK_ROOT`/`DOTD_BIN_DIR`/etc. and `config.LinkRoot/.BinDir/.GeneratedDir` refs → remove.

- [ ] **Step 2: Update `.dagger` fixtures** (at least `test/e2e/fixture/bin/.dagger` + `cmd/dotd/testdata/dotfiles/bin/.dagger` → `$bin`; `cmd/dotd/testdata/dotfiles/config/.dagger` `~/.config` → `$config`; leave `…/conf/.dagger` `link_root: "~"`).

- [ ] **Step 3: Convert Go tests** — per the grep, remove flag args, add `t.Setenv`. Example:
```go
// before: args := []string{"apply", "--link-root", tmp}
t.Setenv("HOME", tmp)
args := []string{"apply"}
```

- [ ] **Step 4: Convert e2e scripts** — replace `--link-root "$X"` (and bin/init/generated flags) with `export HOME="$X"` (+ `export XDG_*` where a route is asserted).

- [ ] **Step 5: Run the full suite**

Run: `go test ./... && go vet ./...`
Expected: PASS, vet clean. Iterate on stragglers.

- [ ] **Step 6: gofmt**

Run: `gofmt -l internal/ cmd/` (expect no output; `gofmt -w` any listed file).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test: fixtures to \$bin/\$config, tests to HOME/XDG env config"
```

---

## Task 6: docs

**Files:** `README.md`, `docs/reference/dotd.md`/`dagger.md`/`annotations.md`/`env-yaml.md`, concepts, `.claude/docs/spec/{symlinks,cli,env}.md`

- [ ] **Step 1: Find references**
```bash
grep -rn 'link_root\|link-root\|~bin\|--config\b\|--env-file\|--bin-dir\|--init-file\|--generated-dir\|bin_dir\|generated_dir\|~/\.config' \
  README.md docs/ .claude/docs/spec/ | grep -v 'superpowers/'
```

- [ ] **Step 2: Update each hit** (same tool's-own-path caution as Task 5):
  - tokens `~`/`$bin`/`$config`; `~bin`→`$bin`; config-route default `~/.config`→`$config`/`$XDG_CONFIG_HOME`.
  - flags: `--config`→`--dotd-config`, `--env-file`→`--dotd-env`; remove `--link-root`/`--bin-dir`/`--init-file`/`--generated-dir` rows.
  - config keys: only `dotfiles` remains; drop `link_root`/`bin_dir`/`generated_dir` rows.
  - document `~`=$HOME always; `$bin`=`($XDG_BIN_HOME ?: ~/.local/bin)/dot-dagger` on PATH; `$config`=$XDG_CONFIG_HOME; the unknown-anchor-token error; the new `dotd paths` command.

- [ ] **Step 3: Commit**
```bash
git add README.md docs/ .claude/docs/spec/
git commit -m "docs: pure-XDG roots model, dotd paths, --dotd-* flags"
```

---

## Task 7: final validation + tracker + PR

- [ ] **Step 1: Full verification**
```bash
go test ./... && go vet ./... && gofmt -l internal/ cmd/
```
Expected: all pass, no gofmt output.

- [ ] **Step 2: Smoke-test the CLI**
```bash
go run ./cmd/dotd paths && \
go run ./cmd/dotd --help 2>&1 | grep -E 'dotd-config|dotd-env' && \
go run ./cmd/dotd --help 2>&1 | grep -vqE 'link-root|bin-dir|init-file|generated-dir' && echo "flags clean"
```
Expected: `dotd paths` prints the resolved table; new flags present; old flags gone.

- [ ] **Step 3: Update trackers** — mark `.claude/TODO.md` 🔴 section DONE; update memory `project_link_root_overhaul.md` to "shipped".

- [ ] **Step 4: Open the PR** (confirm branch not merged first):
```bash
gh pr create --title "feat: pure-XDG roots model — \$bin/\$config tokens, zero path knobs" \
  --body "Implements docs/superpowers/specs/2026-06-13-roots-model-design.md"
```

---

## Self-Review notes

- **Spec coverage:** anchor tokens + expansion (T1); validation/C1 (T1 s9-13); knob-less env resolution via accessors (T2,T3); config.yaml→dotfiles (T3); flag drop + `--dotd-*` rename (T3); `pathFlagOwners` (T3); setup shrink (T3); scaffold `$bin`/`$config` (T3); `$HOME` consumers → `cfg.home` (T3); adopt `$config` (T3); namespaced `$bin` honoring `$XDG_BIN_HOME` (T2); generated/init.sh XDG_DATA (T2/T3); `dotd paths` incl. config/env own paths (T4); tests→env (T5); docs incl. README (T6). Out-of-scope (per-node key, config.yaml removal, tilde-in-value, per-tool relocation) untouched. ✓
- **Compile boundaries:** every commit builds. T1/T2 are self-contained packages; T3 is the atomic `cmd/dotd`+config+adopter cutover (one build/test/commit); T4+ layer additively. Full behavioral green at T5. ✓
- **Type consistency:** `ActOptions.ConfigDir`, `BinPrefix="$bin"`, `ConfigPrefix="$config"`, `ecosystem.Home/XdgBinHome/BinDir/ConfigDir/GeneratedDir/InitFile`, `cfg.home/configDir/binDir/generatedDir/initFile`, `AdoptOptions.HomeDir/ConfigDir`, `newPathsCmd`/`pathRow`, `KeyDotfiles`. `buildActOptions` returns plain `ActOptions`. ✓
- **No placeholders:** every code step shows real code; the test/fixture/doc sweeps (T5,T6) point at authoritative greps — the one place exhaustive enumeration must happen at execution time.
