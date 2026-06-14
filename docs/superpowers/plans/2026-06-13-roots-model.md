# Roots Model (pure-XDG) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the global `link_root` knob with three knob-less anchor tokens (`~`, `$bin`, `$config`) resolved purely from the environment (XDG), remove every path-route/output knob, add anchor-token validation and a `dotd paths` view, and rename the tool's own state-file flags to `--dotd-config`/`--dotd-env`.

**Architecture:** `.dagger` `link_root:`/dest values use anchor tokens; the pipeline stays a pure function receiving resolved `HomeDir`/`BinDir`/`ConfigDir`/`GeneratedDir` in `ActOptions`. All anchors + tool outputs are resolved once in `resolvePaths()` from `ecosystem` accessors (no flag/env/config-field tiers) into `cfg` fields. config.yaml shrinks to a single `dotfiles` field. No backward compatibility — tests configure `HOME`/`XDG_*`.

**Tech Stack:** Go 1.26, cobra/pflag, yaml.v3 (strict `KnownFields`).

**Spec:** `docs/superpowers/specs/2026-06-13-roots-model-design.md`

**Ordering note:** Do tasks in order. Task 1 flips `BinPrefix` (`~bin`→`$bin`), which fixtures (Task 9) depend on; the full suite only goes green after Task 9. Between tasks, expect targeted breakage in not-yet-updated areas — each task states exactly what it must make pass.

---

## File Structure

| File | Change |
|------|--------|
| `internal/pipeline/act.go` | `$bin`/`$config`/`~` `expandDest`; `BinPrefix="$bin"`, `ConfigPrefix="$config"`; `ActOptions.ConfigDir` |
| `internal/pipeline/actions.go` | thread `ConfigDir`; add `validateAnchor` + wire into `validateNode` |
| `internal/ecosystem/ecosystem.go` | `Home()`, `XdgBinHome()`, `BinDir()`, `ConfigDir()`; rename `DefaultGeneratedDir`→`GeneratedDir`, `DefaultInitFile`→`InitFile`; remove `DefaultLinkRoot`, `DefaultBinDir` |
| `internal/config/config.go` | strip to `Dotfiles` only |
| `cmd/dotd/main.go` | cfg fields (`home`,`configDir`; drop `linkRoot`); drop path flags; rename `--dotd-config`/`--dotd-env`; `resolvePaths` from accessors; `pathFlagOwners`; `buildActOptions`; `ValidateNodes` call |
| `cmd/dotd/setup_cmd.go` | strip to `dotfiles` prompt; `Home()` |
| `cmd/dotd/init_cmd.go` | scaffold `$bin`/`$config`; `cfg.home` |
| `cmd/dotd/teardown_cmd.go`, `adopt.go`, `internal/adopter/adopter.go` | `cfg.home`; adopt `ConfigDir` |
| `cmd/dotd/config_cmd.go` | help examples → `dotfiles` |
| `cmd/dotd/paths_cmd.go` (new) | `dotd paths` resolved-view |
| testdata/e2e/docs | `~bin`→`$bin`; `--link-root`→env; flag/key doc updates |

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

- [ ] **Step 3: Update `ActOptions` + token consts** in `internal/pipeline/act.go`. Replace the `ActOptions` struct and `BinPrefix` const (around lines 14-24):

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

- [ ] **Step 4: Rewrite `expandDest`** in `internal/pipeline/act.go` (replace the existing function):

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

Update its caller in `act.go` (the `ActionLink` case in `emitNodeActions`):
```go
			dest := resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```

And the caller in `actions.go` (`CheckLinkConflicts`):
```go
				dest = resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
```

- [ ] **Step 6: Fix the `Act` HomeDir error message** in `act.go` (remove the dead `cfg.linkRoot` reference):
```go
		return nil, fmt.Errorf("act: HomeDir is required — set it to the resolved $HOME")
```

- [ ] **Step 7: Run the expansion test**

Run: `go test ./internal/pipeline/ -run TestExpandDest_Anchors`
Expected: PASS. (Whole-package tests with `~bin` fixtures still fail — fixed in Task 9.)

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

## Task 2: ecosystem accessors

**Files:** Modify `internal/ecosystem/ecosystem.go`; Test `internal/ecosystem/ecosystem_test.go`

- [ ] **Step 1: Write the failing tests** — add to `internal/ecosystem/ecosystem_test.go`:

```go
func TestHome_RespectsHOME(t *testing.T) {
	t.Setenv("HOME", "/home/respected")
	if got, err := Home(); err != nil || got != "/home/respected" {
		t.Fatalf("Home() = %q, %v; want /home/respected", got, err)
	}
}

func TestBinDir_NamespacedHonorsXDG(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_BIN_HOME", "")
	if got, _ := BinDir(); got != "/home/u/.local/bin/"+Name {
		t.Fatalf("BinDir() default = %q", got)
	}
	t.Setenv("XDG_BIN_HOME", "/custom/bin")
	if got, _ := BinDir(); got != "/custom/bin/"+Name {
		t.Fatalf("BinDir() with XDG_BIN_HOME = %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ecosystem/ -run 'TestHome_RespectsHOME|TestBinDir_NamespacedHonorsXDG'`
Expected: FAIL — `Home`, `BinDir` undefined.

- [ ] **Step 3: Replace `DefaultLinkRoot` and `DefaultBinDir`** in `internal/ecosystem/ecosystem.go`. Remove both functions; add:

```go
// Home returns the user's home directory ($HOME on linux/darwin) — the single
// canonical accessor for "~". Not a configurable knob: $HOME is authoritative
// (universal convention, like $EDITOR).
func Home() (string, error) {
	return userHome()
}

// XdgBinHome returns $XDG_BIN_HOME if set to an absolute path, else ~/.local/bin.
// $XDG_BIN_HOME is not part of the XDG base spec but is the de-facto convention
// for user binaries; honoring it lets users relocate the bin root the standard
// (system-wide) way.
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
// Namespacing is free because PATH is a search list; init.sh adds this dir to PATH.
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

- [ ] **Step 4: Rename `DefaultGeneratedDir`→`GeneratedDir` and `DefaultInitFile`→`InitFile`** (the path-output resolvers — keep their bodies, drop the "Default" prefix and adjust the doc comment to drop "default"). Leave `DefaultConfigFile`, `DefaultEnvFile`, `DefaultDotfiles` untouched (still used by the surviving configurable paths).

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ecosystem/ -run 'TestHome|TestBinDir'`
Expected: PASS. (Package compiles; cmd refs to the removed/renamed functions are fixed in Task 4.)

- [ ] **Step 6: Commit**

```bash
git add internal/ecosystem/ecosystem.go internal/ecosystem/ecosystem_test.go
git commit -m "feat(ecosystem): Home/XdgBinHome/BinDir/ConfigDir accessors"
```

---

## Task 3: config.yaml schema — `dotfiles` only

**Files:** Modify `internal/config/config.go`; Test `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test** — add to `internal/config/config_test.go`:

```go
func TestConfig_OnlyDotfiles(t *testing.T) {
	if len(Keys) != 1 || Keys[0] != KeyDotfiles {
		t.Fatalf("Keys = %v, want [dotfiles]", Keys)
	}
	if _, err := loadFrom(strings.NewReader("bin_dir: /x\n")); err == nil {
		t.Fatal("expected strict-decode error for removed bin_dir field")
	}
	if _, err := loadFrom(strings.NewReader("link_root: ~/.config\n")); err == nil {
		t.Fatal("expected strict-decode error for removed link_root field")
	}
}
```

Ensure `strings` is imported in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestConfig_OnlyDotfiles`
Expected: FAIL — `bin_dir`/`link_root` currently decode fine; `Keys` has 4 entries.

- [ ] **Step 3: Strip `internal/config/config.go`** to a single field. Replace the const block, `Keys`, struct, and the `Get`/`Set` switch bodies:

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

**No special-case handling** for removed fields — strict `KnownFields(true)` rejecting them as unknown is exactly the desired behavior.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): shrink config.yaml to dotfiles only"
```

---

## Task 4: cmd wiring — flags, resolvePaths, ActOptions

**Files:** Modify `cmd/dotd/main.go`; callers `unapply_cmd.go`, `compose_cmd.go`; Test `cmd/dotd/main_test.go`

- [ ] **Step 1: Write the failing test** — add to `cmd/dotd/main_test.go`:

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
	if cfg.home != home {
		t.Errorf("home = %q, want %q", cfg.home, home)
	}
	if cfg.configDir != "/xdg/conf" {
		t.Errorf("configDir = %q", cfg.configDir)
	}
	if cfg.binDir != "/xdg/bin/dot-dagger" {
		t.Errorf("binDir = %q", cfg.binDir)
	}
}
```

Ensure `path/filepath` is imported.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotd/ -run TestResolvePaths_AnchorsFromEnv`
Expected: FAIL — `cfg.home`/`cfg.configDir` undefined.

- [ ] **Step 3: Edit the cfg struct** in `cmd/dotd/main.go` (the `appConfig` struct). Replace `linkRoot string` with `home string` and add `configDir string`; keep `binDir`, `generatedDir`, `initFile`, `envFile`, `configPath`, `files`:

```go
	files        string
	configPath   string
	envFile      string
	env          []string
	initFile     string
	home         string
	configDir    string
	binDir       string
	generatedDir string
```

- [ ] **Step 4: Edit flag registrations** in `newRootCmd`. Rename `--config`/`--env-file`, and **delete** the `--init-file`, `--link-root`, `--bin-dir`, `--generated-dir` lines:

```go
	pf.StringVar(&cfg.configPath, "dotd-config", "", "path to dot-dagger's own config.yaml (default: $DOTD_CONFIG_FILE → ~/.config/dot-dagger/config.yaml)")
	pf.StringVar(&cfg.envFile, "dotd-env", "", fmt.Sprintf("path to dot-dagger's own %s (default: $DOTD_ENV_FILE → ~/.config/dot-dagger/%s)", ecosystem.EnvFileName, ecosystem.EnvFileName))
```

(Leave `--files`, `--env`, `--dry-run`, `--force`, log flags as-is.)

- [ ] **Step 5: Edit `pathFlagOwners`** — remove the `init-file`, `link-root`, `bin-dir`, `generated-dir` entries entirely (those flags no longer exist). Keep only `dry-run` and `force`:

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

- [ ] **Step 6: Edit `resolvePaths`** in `cmd/dotd/main.go`. Replace the `cfg.initFile`, `cfg.linkRoot`, `cfg.binDir`, `cfg.generatedDir` `ResolvePath` blocks with direct accessor calls (no flag/env/config tiers), and add `home`/`configDir`:

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

Leave the `cfg.envFile` (`DOTD_ENV_FILE`), `cfg.configPath` (`DOTD_CONFIG_FILE`), and `cfg.files` resolutions intact. The `toolCfg` now only carries `Dotfiles`; update the `filesFromCwd` line and the `cfg.files` ResolvePath which reference `toolCfg.Dotfiles` — those are unchanged (still valid).

- [ ] **Step 7: Update `buildActOptions`** in `cmd/dotd/main.go` (no error return needed — fields are pre-resolved):

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

- [ ] **Step 8: Update the `ValidateNodes` call** in `cmd/dotd/main.go`:

```go
	if err := pipeline.ValidateNodes(nodes, pipeline.ActOptions{HomeDir: cfg.home, BinDir: cfg.binDir, ConfigDir: cfg.configDir}); err != nil {
		return nil, 0, 0, err
	}
```

- [ ] **Step 9: Build the package + run the resolve test**

Run: `go build ./cmd/dotd/ && go test ./cmd/dotd/ -run TestResolvePaths_AnchorsFromEnv`
Expected: build OK (any leftover `cfg.linkRoot` reference is a compile error — fix it to `cfg.home` in `init_cmd.go`/`teardown_cmd.go`/`adopt.go` per Tasks 6-7; for THIS task just ensure main.go compiles — those files are touched next, so it's acceptable to do their `cfg.linkRoot`→`cfg.home` rename now if the build blocks). PASS on the resolve test.

- [ ] **Step 10: Commit**

```bash
git add cmd/dotd/main.go cmd/dotd/main_test.go
git commit -m "feat(cmd): resolve anchors from env, drop path knobs, rename --dotd-* flags"
```

---

## Task 5: setup wizard — dotfiles only

**Files:** Modify `cmd/dotd/setup_cmd.go`

- [ ] **Step 1: Switch the home read** (top of `runSetup`) from `os.UserHomeDir()` to `ecosystem.Home()`:
```go
	home, err := ecosystem.Home()
	if err != nil {
		return err
	}
```

- [ ] **Step 2: Simplify `isUpdate`** — only `Dotfiles` remains:
```go
	isUpdate := existing.Dotfiles != ""
```

- [ ] **Step 3: Delete the `binDir`, `generatedDir`, `linkRoot` prompts** (the three `promptPath` blocks for "Bin directory", "Generated files directory", "Link root"). Keep only the `dotfilesPath` prompt.

- [ ] **Step 4: Simplify the written `toolCfg`**:
```go
	toolCfg := &dotcfg.Config{Dotfiles: dotfilesPath}
```

- [ ] **Step 5: Build + run setup tests**

Run: `go test ./cmd/dotd/ -run Setup`
Expected: setup tests that fed bin/generated/link-root prompt lines must drop those input lines and stop asserting those config keys. Update + PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/setup_cmd.go cmd/dotd/*_test.go
git commit -m "feat(setup): wizard prompts for dotfiles repo only"
```

---

## Task 6: init — scaffold tokens + source-line home

**Files:** Modify `cmd/dotd/init_cmd.go`

- [ ] **Step 1: Source-line home.** In `maybeAddSourceLine`, after the `shell == ""` guard, resolve home and use it instead of `cfg.linkRoot`:
```go
	home, err := ecosystem.Home()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
```
Change the `DetectShellConfig(shell, resolved["os"], cfg.linkRoot)` call to `…, home)` and `AppendSourceLine(sc.RCFile, cfg.initFile, cfg.linkRoot)` to `…, home)`. (Use `=`/scoping so the existing `err` isn't redeclared incorrectly.)

- [ ] **Step 2: Update scaffold tokens** in `conventionRoles`. The "Config files" entry:
```go
	{
		label:   "Config files",
		desc:    "Files here are symlinked into your config dir (e.g. config/nvim/init.lua → $config/nvim/init.lua, i.e. $XDG_CONFIG_HOME).",
		defDir:  adopter.DirConfig,
		content: "link_root: \"" + pipeline.ConfigPrefix + "\"\ndefaults:\n  actions:\n    - link\n",
	},
```
The "Bin scripts" entry already uses `pipeline.BinPrefix` (now `$bin`) — verify it renders `link_root: "$bin"`; no edit needed.

- [ ] **Step 3: Build + run init/scaffold tests**

Run: `go test ./cmd/dotd/ -run 'Init|Scaffold'`
Expected: scaffold-content assertions must expect `link_root: "$config"` / `"$bin"`. Update + PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/dotd/init_cmd.go cmd/dotd/*_test.go
git commit -m "feat(init): scaffold \$bin/\$config, source line uses Home()"
```

---

## Task 7: teardown + adopt

**Files:** Modify `cmd/dotd/teardown_cmd.go`, `cmd/dotd/adopt.go`, `internal/adopter/adopter.go`

- [ ] **Step 1: teardown** — before the `DetectShellConfig(shell, osName, cfg.linkRoot)` call, resolve home and pass it:
```go
	home, err := ecosystem.Home()
	if err != nil {
		return err
	}
```
Change that call's last arg to `home`. (Confirm the enclosing function returns `error`.)

- [ ] **Step 2: adopter struct** — in `internal/adopter/adopter.go`, rename `LinkRoot` → `HomeDir` and add `ConfigDir`:
```go
	HomeDir   string // resolved real $HOME (for "~" expansion)
	ConfigDir string // resolved config route (for "$config" expansion)
	BinDir    string // resolved bin route
```

- [ ] **Step 3: adopter ActOptions** — update the build:
```go
	actOpts := pipeline.ActOptions{
		HomeDir:   opts.HomeDir,
		BinDir:    opts.BinDir,
		ConfigDir: opts.ConfigDir,
		DryRun:    opts.DryRun,
		Force:     opts.Force,
	}
```

- [ ] **Step 4: adopt cmd** — in `cmd/dotd/adopt.go`, resolve home and populate the renamed/new fields:
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
(If `err` is already declared in `runAdopt`, use `=`.)

- [ ] **Step 5: Build + run tests**

Run: `go test ./internal/adopter/ ./cmd/dotd/ -run 'Adopt|Teardown'`
Expected: adopter tests setting `LinkRoot:` → `HomeDir:`. Update + PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/dotd/teardown_cmd.go cmd/dotd/adopt.go internal/adopter/adopter.go internal/adopter/*_test.go
git commit -m "feat(adopt,teardown): use Home(), adopt resolves \$config"
```

---

## Task 8: `dotd paths` view + config_cmd help

**Files:** Create `cmd/dotd/paths_cmd.go`; Modify `cmd/dotd/config_cmd.go`, `cmd/dotd/main.go` (register command); Test `cmd/dotd/paths_cmd_test.go`

- [ ] **Step 1: Write the failing test** — `cmd/dotd/paths_cmd_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPathsCmd_ShowsResolvedAnchors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &config{home: home, binDir: home + "/.local/bin/dot-dagger", configDir: home + "/.config", generatedDir: home + "/gen", initFile: home + "/init.sh", files: home + "/dotfiles"}
	var out bytes.Buffer
	cmd := newPathsCmd(cfg)
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"$bin", "$config", "home", "init.sh", "dotfiles", home} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("paths output missing %q:\n%s", want, out.String())
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotd/ -run TestPathsCmd_ShowsResolvedAnchors`
Expected: FAIL — `newPathsCmd` undefined.

- [ ] **Step 3: Create `cmd/dotd/paths_cmd.go`**:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

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
			rows := []struct{ Name, Path string }{
				{"home", cfg.home},
				{"$bin", cfg.binDir},
				{"$config", cfg.configDir},
				{"generated", cfg.generatedDir},
				{"init.sh", cfg.initFile},
				{"dotfiles", cfg.files},
			}
			if jsonOutput {
				return writePathsJSON(cmd.OutOrStdout(), rows)
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", r.Name, r.Path)
			}
			return nil
		},
	}
	addJSONFlag(cmd, &jsonOutput)
	return cmd
}
```

Add the JSON helper in the same file (mirrors `config show`'s JSON shape):
```go
import "encoding/json" // add to the import block

func writePathsJSON(w io.Writer, rows []struct{ Name, Path string }) error {
	type entry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	out := make([]entry, len(rows))
	for i, r := range rows {
		out[i] = entry{Name: r.Name, Path: r.Path}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```
(Add `"io"` to imports. Verify `addJSONFlag` exists — it is used by `config show` in `config_cmd.go`; reuse it.)

- [ ] **Step 4: Register the command** in `cmd/dotd/main.go` where other subcommands are added (alongside `newConfigCmd(cfg)` etc.):
```go
	root.AddCommand(newPathsCmd(cfg))
```
Assign it the `config` group if commands set `GroupID` there (match the surrounding registration pattern).

- [ ] **Step 5: Update `config_cmd.go` help examples** — replace `link_root` references with `dotfiles`:
  - `dotd config get link_root` → `dotd config get dotfiles`
  - `dotd config set link_root /home/me` → `dotd config set dotfiles ~/dotfiles`

- [ ] **Step 6: Run the test + build**

Run: `go test ./cmd/dotd/ -run TestPathsCmd_ShowsResolvedAnchors && go build ./cmd/dotd/`
Expected: PASS + build OK.

- [ ] **Step 7: Commit**

```bash
git add cmd/dotd/paths_cmd.go cmd/dotd/paths_cmd_test.go cmd/dotd/main.go cmd/dotd/config_cmd.go
git commit -m "feat(cmd): add 'dotd paths' resolved-view"
```

---

## Task 9: fixtures + tests — `~bin`→`$bin`, flags→env. Makes the suite green.

**Files:** testdata `.dagger` fixtures + Go tests + e2e scripts

- [ ] **Step 1: Enumerate the blast radius first.** Run and read before editing:

```bash
grep -rn '~bin\|~/\.config\|--link-root\|link-root\|LinkRoot\|DOTD_LINK_ROOT\|link_root\|--bin-dir\|bin-dir\|--init-file\|init-file\|--generated-dir\|generated-dir\|DOTD_BIN_DIR\|DOTD_INIT_FILE\|DOTD_GENERATED_DIR' \
  internal/ cmd/ test/ --include='*.go' --include='*.dagger' --include='*.sh' --include='*.yaml'
```
Triage each hit:
  - per-node `.dagger` `link_root:` value `~` or abs path → leave.
  - value `~bin` → `$bin`; config-route `~/.config` in a fixture exercising config linking → `$config` (leave literal `~/.config` only where a test asserts `$HOME/.config` semantics).
  - `--link-root <d>`/`--bin-dir <d>`/`--init-file <f>`/`--generated-dir <d>` in a Go test → remove the flag arg; add `t.Setenv("HOME", d)` and/or `t.Setenv("XDG_BIN_HOME"/"XDG_DATA_HOME"/"XDG_CONFIG_HOME", …)` as the test requires.
  - same flags in e2e `.sh` → remove; `export HOME=…` (+ `XDG_*`) near the top.
  - `DOTD_LINK_ROOT`/`DOTD_BIN_DIR`/etc. and `config.LinkRoot`/`.BinDir`/`.GeneratedDir` refs → remove.

- [ ] **Step 2: Update `.dagger` fixtures** per the grep (at least: `test/e2e/fixture/bin/.dagger`, `cmd/dotd/testdata/dotfiles/bin/.dagger` → `$bin`; `cmd/dotd/testdata/dotfiles/config/.dagger` `~/.config` → `$config`; leave `…/conf/.dagger` `link_root: "~"`).

- [ ] **Step 3: Convert Go tests** — for each hit, remove the flag from the args slice, add the `t.Setenv(...)`. Example:
```go
// before: args := []string{"apply", "--link-root", tmp}
t.Setenv("HOME", tmp)
args := []string{"apply"}
```

- [ ] **Step 4: Convert e2e scripts** — replace `--link-root "$X"` (and bin/init/generated flags) with `export HOME="$X"` (+ `export XDG_*` where a route is asserted) near the fake-home setup.

- [ ] **Step 5: Run the full suite**

Run: `go test ./... && go vet ./...`
Expected: PASS, vet clean. Iterate on stragglers until green.

- [ ] **Step 6: gofmt**

Run: `gofmt -l internal/ cmd/` (expect no output; `gofmt -w` any listed file).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test: fixtures to \$bin/\$config, tests to HOME/XDG env config"
```

---

## Task 10: docs

**Files:** `README.md`, `docs/reference/dotd.md`/`dagger.md`/`annotations.md`/`env-yaml.md`, concepts, `.claude/docs/spec/{symlinks,cli,env}.md`

- [ ] **Step 1: Find references**
```bash
grep -rn 'link_root\|link-root\|~bin\|--config\b\|--env-file\|--bin-dir\|--init-file\|--generated-dir\|bin_dir\|generated_dir\|~/\.config' \
  README.md docs/ .claude/docs/spec/ | grep -v 'superpowers/'
```

- [ ] **Step 2: Update each hit:**
  - tokens `~`/`$bin`/`$config`; `~bin`→`$bin`; config-route default `~/.config`→`$config`/`$XDG_CONFIG_HOME`.
  - flags: `--config`→`--dotd-config`, `--env-file`→`--dotd-env`; remove `--link-root`/`--bin-dir`/`--init-file`/`--generated-dir` rows.
  - config keys: only `dotfiles` remains; drop `link_root`/`bin_dir`/`generated_dir` rows.
  - document: `~`=$HOME always; `$bin`=`($XDG_BIN_HOME ?: ~/.local/bin)/dot-dagger` on PATH; `$config`=$XDG_CONFIG_HOME; the unknown-anchor-token error; the new `dotd paths` command.

- [ ] **Step 3: Commit**
```bash
git add README.md docs/ .claude/docs/spec/
git commit -m "docs: pure-XDG roots model, dotd paths, --dotd-* flags"
```

---

## Task 11: final validation + tracker + PR

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

- **Spec coverage:** anchor tokens + expansion (T1); validation/C1 (T1 s9-13); knob-less env resolution via accessors (T2,T4); config.yaml→dotfiles (T3); flag drop + `--dotd-*` rename (T4); `pathFlagOwners` cleanup (T4); setup shrink (T5); scaffold `$bin`/`$config` (T6); `$HOME` consumers → `cfg.home` (T4,T6,T7); adopt `$config` (T7); `dotd paths` view (T8); namespaced `$bin` honoring `$XDG_BIN_HOME` (T2); generated/init.sh XDG_DATA (T2 via GeneratedDir/InitFile, resolved T4); tests→env (T9); docs incl. README (T10). Out-of-scope (per-node key, config.yaml removal, tilde-in-value, per-tool relocation) untouched. ✓
- **Type consistency:** `ActOptions.ConfigDir`, `BinPrefix="$bin"`, `ConfigPrefix="$config"`, `ecosystem.Home/XdgBinHome/BinDir/ConfigDir/GeneratedDir/InitFile`, `cfg.home/configDir/binDir/generatedDir/initFile`, `AdoptOptions.HomeDir/ConfigDir`, `newPathsCmd`, `KeyDotfiles` used consistently. `buildActOptions` returns plain `ActOptions` (no error — fields pre-resolved in `resolvePaths`). ✓
- **No placeholders:** every code step shows real code; the test/fixture sweep (T9) and doc sweep (T10) point at authoritative greps — the one place exhaustive enumeration must happen at execution time, by design.
