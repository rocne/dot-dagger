# v2 Env & Config System Implementation Plan

> **ROUGH DRAFT** — flesh out code blocks before executing.
>
> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the env and config systems: `env.yaml` with `$(...)` shell interpolation, `config.yaml` for tool preferences, env resolution order, hidden internal commands (`dotd get-os`, `dotd get-hostname`), and `dotd env *` / `dotd config *` subcommands.

**Architecture:** Two internal packages (`internal/env/` rewrite, new `internal/config/`) plus new CLI subcommands. The env package handles resolution order and shell interpolation. The config package manages the tool's own preferences file. Both are independent of the pipeline — Plan 3 will call them.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `os/exec` for shell interpolation, cobra for CLI.

**Prerequisite:** Plan 1 (Foundation) complete.

---

## Context

v2 drops hardcoded auto-detectors (`os`, `distro`, `shell`). Instead, `env.yaml` supports `$(...)` shell expressions. Users call `$(dotd get-os)`, `$(dotd get-hostname)`, or any shell command. `dotd get-os`/`dotd get-hostname` are hidden commands shipped with the binary.

Config file locations: `~/.config/dot-dagger/config.yaml` and `~/.config/dot-dagger/env.yaml`.

Env resolution order (highest → lowest):
1. `--env key=val,key2=val2` CLI flags
2. `DOTD_*` shell env vars (e.g. `DOTD_CONTEXT=work`)
3. `env.yaml` values (static or `$(...)` evaluated)
4. Prompt (TTY) or halt (non-interactive)

---

## File Map

| File | Status | Responsibility |
|------|--------|----------------|
| `internal/env/env.go` | Rewrite | Load `env.yaml`, evaluate `$(...)`, merge resolution order |
| `internal/env/env_test.go` | Rewrite | Full test suite |
| `internal/env/detect.go` | Delete | Replaced by `$(dotd get-os)` etc. in env.yaml |
| `internal/config/config.go` | Create | Load/save `config.yaml`; get/set individual keys |
| `internal/config/config_test.go` | Create | Parser and get/set tests |
| `cmd/dotd/getters.go` | Create | Hidden `get-os` and `get-hostname` commands |
| `cmd/dotd/env_cmd.go` | Create | `dotd env show/get/set/edit` subcommands |
| `cmd/dotd/config_cmd.go` | Create | `dotd config show/get/set/edit` subcommands |
| `cmd/dotd/main.go` | Modify | Register new subcommands; add `--all` flag to help |

---

## Task 1: Rewrite `internal/env/`

The new env package:
- Loads `env.yaml` where values may be `$(shell command)` expressions
- Evaluates `$(...)` by running the shell command via `sh -c`
- Merges with `DOTD_*` env vars and `--env` overrides
- Returns `map[string]string` of resolved values

### New `env.yaml` format

```yaml
os: $(dotd get-os)
hostname: $(dotd get-hostname)
context: work
my_gpu: $(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null || echo none)
```

### `internal/env/env.go` key types and functions

```go
// Load parses env.yaml from path. Returns raw map[string]string (unexpanded).
func Load(path string) (map[string]string, error)

// Expand evaluates $(…) expressions in raw values. Uses sh -c.
// Failed commands → empty string (treated as missing by Resolve).
func Expand(raw map[string]string) (map[string]string, error)

// Resolve merges layers in precedence order.
// cliFlags: parsed from --env flag. shellVars: DOTD_* vars from os.Environ().
// expanded: result of Expand(Load(envYamlPath)).
func Resolve(cliFlags, shellVars, expanded map[string]string) map[string]string

// ParseFlags parses "key=val,key2=val2" into a map.
func ParseFlags(s string) (map[string]string, error)

// ShellVars extracts DOTD_* vars from environ, lowercasing the suffix as the key.
// DOTD_CONTEXT=work → context=work
func ShellVars(environ []string) map[string]string
```

- [ ] **Step 1.1: Write tests for `Load`, `Expand`, `Resolve`, `ParseFlags`, `ShellVars`**

Tests cover:
- `Load` on a YAML file with static values and `$(...)` expressions
- `Expand` on a map with `$(echo hello)` → `"hello"`, failed command → `""`
- `Resolve` precedence: cliFlags win over shellVars win over expanded
- `ParseFlags("context=work,os=linux")` → `{"context":"work","os":"linux"}`
- `ShellVars([]string{"DOTD_CONTEXT=work","HOME=/home/x"})` → `{"context":"work"}`

- [ ] **Step 1.2: Run tests to verify they fail**

```bash
cd /home/rocne/git/dot-dagger && go test ./internal/env/... -v 2>&1 | head -30
```

- [ ] **Step 1.3: Implement `internal/env/env.go`**

Key implementation notes:
- `Expand`: for each value, check if it matches `^\$\((.+)\)$`; if so, run `sh -c "<cmd>"` and capture stdout (trimmed)
- `ShellVars`: iterate `os.Environ()`, match `DOTD_` prefix, lowercase suffix
- `Resolve`: merge maps in order: expanded → shellVars → cliFlags (later wins)

- [ ] **Step 1.4: Delete `internal/env/detect.go`**

```bash
git rm internal/env/detect.go
```

- [ ] **Step 1.5: Run tests and commit**

```bash
go test ./internal/env/... -v
git add internal/env/
git commit -m "feat(env): rewrite for v2 shell interpolation and resolution order"
```

---

## Task 2: Create `internal/config/`

`config.yaml` holds tool preferences — machine-stable settings that don't change per context.

```yaml
dotfiles: ~/dotfiles
bin_dir: ~/bin
generated_dir: ~/.config/dot-dagger/generated
link_root: ~
```

### Key types and functions

```go
// Config holds parsed config.yaml values.
type Config struct {
    Dotfiles     string `yaml:"dotfiles"`
    BinDir       string `yaml:"bin_dir"`
    GeneratedDir string `yaml:"generated_dir"`
    LinkRoot     string `yaml:"link_root"`
}

// DefaultPath returns the default config.yaml path: ~/.config/dot-dagger/config.yaml
func DefaultPath() (string, error)

// Load parses config.yaml at path. Returns zero-value Config if file not found.
func Load(path string) (*Config, error)

// Save writes Config to path, creating parent dirs if needed.
func Save(path string, cfg *Config) error

// Get returns the value of a key by name. Error if key unknown.
func (c *Config) Get(key string) (string, error)

// Set updates a key by name. Error if key unknown. Returns updated Config.
func (c *Config) Set(key, value string) error
```

Supported keys: `dotfiles`, `bin_dir`, `generated_dir`, `link_root`.

- [ ] **Step 2.1: Write tests**

Tests cover:
- `Load` on real YAML file; `Load` on non-existent file → zero value, no error
- `Get("dotfiles")` returns value; `Get("unknown")` returns error
- `Set("link_root", "~/apps")` updates field; `Set("unknown", "x")` returns error
- Round-trip: Load → Set → Save → Load returns updated value

- [ ] **Step 2.2: Run tests to verify they fail**

```bash
go test ./internal/config/... -v 2>&1 | head -20
```

- [ ] **Step 2.3: Implement `internal/config/config.go`**

`Get`/`Set` use reflection or a switch over known field names — switch is simpler and avoids reflection.

- [ ] **Step 2.4: Run tests and commit**

```bash
go test ./internal/config/... -v
git add internal/config/
git commit -m "feat(config): add config.yaml loader with get/set"
```

---

## Task 3: Hidden Internal Commands

`dotd get-os` and `dotd get-hostname` are registered as cobra commands with `Hidden: true`. They print a single normalized string to stdout and exit.

**`dotd get-os` output values:** `macos`, `linux`, `windows` (lowercased, normalized from `runtime.GOOS`)

**`dotd get-hostname` output:** result of `os.Hostname()`

- [ ] **Step 3.1: Create `cmd/dotd/getters.go`**

```go
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var getOSCmd = &cobra.Command{
	Use:    "get-os",
	Hidden: true,
	Short:  "Print normalized OS name",
	RunE: func(cmd *cobra.Command, args []string) error {
		goos := runtime.GOOS
		switch goos {
		case "darwin":
			goos = "macos"
		}
		fmt.Println(strings.ToLower(goos))
		return nil
	},
}

var getHostnameCmd = &cobra.Command{
	Use:    "get-hostname",
	Hidden: true,
	Short:  "Print system hostname",
	RunE: func(cmd *cobra.Command, args []string) error {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		fmt.Println(h)
		return nil
	},
}
```

- [ ] **Step 3.2: Register in `main.go` and verify `dotd help` hides them, `dotd help --all` shows them**

Add `rootCmd.AddCommand(getOSCmd, getHostnameCmd)` in `main.go`.

Add `--all` flag to help: when set, temporarily set `Hidden: false` on all commands before printing help. Use `cobra.Command.InitDefaultHelpCmd()` or a custom help function.

- [ ] **Step 3.3: Commit**

```bash
git add cmd/dotd/getters.go cmd/dotd/main.go
git commit -m "feat(cli): add hidden get-os and get-hostname commands"
```

---

## Task 4: `dotd env` Subcommands

```
dotd env show              — print all resolved env key=value pairs
dotd env get <key>         — print single value; exit 1 if missing
dotd env set <key> <value> — write to env.yaml (static value)
dotd env edit              — open env.yaml in $EDITOR
```

- [ ] **Step 4.1: Create `cmd/dotd/env_cmd.go`**

`dotd env show`: calls `env.Load` + `env.Expand`, merges with `ShellVars(os.Environ())`, prints sorted `key=value` pairs.

`dotd env get <key>`: same resolution, print value or exit 1 with error message.

`dotd env set <key> <value>`: loads raw YAML, sets key as static string (no `$(...)`), saves.

`dotd env edit`: `exec.Command(editor, envYamlPath).Run()` where `editor = os.Getenv("EDITOR")`, default `vi`.

- [ ] **Step 4.2: Register `envCmd` as subcommand in `main.go`**

- [ ] **Step 4.3: Manual smoke test**

```bash
go run ./cmd/dotd env show
go run ./cmd/dotd env set context work
go run ./cmd/dotd env get context   # should print: work
```

- [ ] **Step 4.4: Commit**

```bash
git add cmd/dotd/env_cmd.go
git commit -m "feat(cli): add dotd env subcommands"
```

---

## Task 5: `dotd config` Subcommands

```
dotd config show              — print all config key=value pairs
dotd config get <key>         — print single value
dotd config set <key> <value> — write to config.yaml
dotd config edit              — open config.yaml in $EDITOR
```

- [ ] **Step 5.1: Create `cmd/dotd/config_cmd.go`**

Mirrors env_cmd.go but uses `internal/config` package. `show` prints all fields. `get`/`set` use `Config.Get`/`Config.Set`.

- [ ] **Step 5.2: Register and smoke test**

```bash
go run ./cmd/dotd config show
go run ./cmd/dotd config set link_root ~/apps
go run ./cmd/dotd config get link_root  # should print: ~/apps
```

- [ ] **Step 5.3: Commit**

```bash
git add cmd/dotd/config_cmd.go
git commit -m "feat(cli): add dotd config subcommands"
```
