// Package setup scaffolds a dotfiles repo structure and config files for first-time use.
// All operations are idempotent: existing files and directories are skipped without error.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/packages"
)

// Options configures the scaffold operation.
type Options struct {
	DotfilesDir  string
	EnvFilePath  string
	InitFilePath string
	// Detected values shown as comments in the generated env.yaml.
	DetectedOS     string
	DetectedDistro string
	DetectedShell  string
	// SelectedManagers is the list of package manager names to pre-fill in packages.yaml.
	// If nil, packages.yaml is generated with no manager entries (comment-only).
	SelectedManagers []string
}

// ActionKind describes what kind of thing was created.
type ActionKind string

const (
	KindDir  ActionKind = "dir"
	KindFile ActionKind = "file"
)

// ActionState describes the outcome of a setup action.
type ActionState string

const (
	StateCreated ActionState = "created"
	StateSkipped ActionState = "skipped"
)

// Action is one step taken (or skipped) during scaffold.
type Action struct {
	Kind  ActionKind
	Path  string
	State ActionState
}

// Result holds the actions taken during scaffold.
type Result struct {
	Actions []Action
}

// Scaffold creates the dotfiles repo structure and config files.
// Idempotent: existing files and directories are skipped without error.
func Scaffold(opts Options) (*Result, error) {
	var res Result

	// Repo directories owned by suite tools.
	for _, sub := range []string{"shellrc", "conf", "bin"} {
		act, err := ensureDir(filepath.Join(opts.DotfilesDir, sub))
		if err != nil {
			return nil, err
		}
		res.Actions = append(res.Actions, act)
	}

	// Config files — only written if absent.
	files := []struct {
		path    string
		content string
	}{
		{opts.EnvFilePath, envYAML(opts)},
		{filepath.Join(opts.DotfilesDir, ecosystem.ConfigFile), daggerYAML()},
		{filepath.Join(opts.DotfilesDir, "packages.yaml"), packagesYAML(opts.SelectedManagers)},
	}
	for _, f := range files {
		act, err := ensureFile(f.path, f.content)
		if err != nil {
			return nil, err
		}
		res.Actions = append(res.Actions, act)
	}

	return &res, nil
}

func ensureDir(path string) (Action, error) {
	if _, err := os.Stat(path); err == nil {
		return Action{Kind: KindDir, Path: path, State: StateSkipped}, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Action{}, fmt.Errorf("setup: create dir %s: %w", path, err)
	}
	return Action{Kind: KindDir, Path: path, State: StateCreated}, nil
}

func ensureFile(path, content string) (Action, error) {
	if _, err := os.Stat(path); err == nil {
		return Action{Kind: KindFile, Path: path, State: StateSkipped}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Action{}, fmt.Errorf("setup: mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Action{}, fmt.Errorf("setup: write %s: %w", path, err)
	}
	return Action{Kind: KindFile, Path: path, State: StateCreated}, nil
}

// --- file content generators ---

func envYAML(opts Options) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s environment — keys used in @when predicates.\n", ecosystem.Name)
	sb.WriteString("# Built-in keys are auto-detected at runtime. Uncomment to override.\n")
	if opts.DetectedOS != "" || opts.DetectedDistro != "" || opts.DetectedShell != "" {
		sb.WriteString("\n")
	}
	if opts.DetectedOS != "" {
		fmt.Fprintf(&sb, "# os: %s\n", opts.DetectedOS)
	}
	if opts.DetectedDistro != "" {
		fmt.Fprintf(&sb, "# distro: %s\n", opts.DetectedDistro)
	}
	if opts.DetectedShell != "" {
		fmt.Fprintf(&sb, "# shell: %s\n", opts.DetectedShell)
	}
	sb.WriteString(`
# dotfiles_repo: ~/dotfiles   # path to dotfiles repo (overrides $DOTFILES / cwd)
# link_root: ~                # symlink root for conf/ files (default: $HOME)
# bin_dir: ~/.local/bin/dot-dagger  # destination for bin/ files
# generated_dir: ~/.local/share/dot-dagger/generated  # compose output directory
# init_file: ~/.local/share/dot-dagger/init.sh        # generated shell init file
`)
	return sb.String()
}

func daggerYAML() string {
	return `# Per-directory config for this directory.
# All fields are optional.

# when: "os=linux"           # predicate: include this dir only when true
# link_root: "~"             # symlink base for link actions (relative extends parent)
# actions:
#   - source                 # default action for files in this dir
#   - no-source

# defaults:                  # cascades to all files in this dir
#   when: "context=work"
#   actions:
#     - source

# files:                     # explicit config for non-annotatable files (JSON, Lua, etc.)
#   settings.json:
#     name: nvim.settings
#     actions:
#       - link(~/.config/nvim/settings.json)
`
}

func packagesYAML(selected []string) string {
	var sb strings.Builder

	sb.WriteString("# Package registry — maps logical package names to package manager entries.\n")
	sb.WriteString("# Used by @require (hard gate) and @request (soft ask) annotations.\n")
	sb.WriteString("#\n")
	sb.WriteString("# package_managers: default install/uninstall/update command templates.\n")
	sb.WriteString("#   {package} is substituted with the package name at runtime.\n")
	sb.WriteString("#\n")
	sb.WriteString("# packages: each key is the logical name used in @require/@request.\n")
	sb.WriteString("#   binary:   executable checked by installed() — defaults to the logical name.\n")
	sb.WriteString("#   managers: which managers can install this package.\n")
	sb.WriteString("#     empty entry = use manager defaults, pass logical name as package arg.\n")
	sb.WriteString("#     package:    = override the name passed to this manager's CLI.\n")

	// Build a name→def lookup from the catalog.
	catalog := make(map[string]packages.KnownManager, len(packages.Catalog))
	for _, m := range packages.Catalog {
		catalog[m.Name] = m
	}

	if len(selected) > 0 {
		sb.WriteString("\npackage_managers:\n")
		// priority: ordered preference list
		sb.WriteString("  priority:")
		for _, name := range selected {
			fmt.Fprintf(&sb, " %s", name)
		}
		sb.WriteString("\n")
		// per-manager command templates
		for _, name := range selected {
			m, ok := catalog[name]
			if !ok {
				continue
			}
			fmt.Fprintf(&sb, "  %s:\n", name)
			fmt.Fprintf(&sb, "    install: %q\n", m.Def.Install)
			fmt.Fprintf(&sb, "    uninstall: %q\n", m.Def.Uninstall)
			fmt.Fprintf(&sb, "    update: %q\n", m.Def.Update)
		}
	} else {
		sb.WriteString("#\n# package_managers:\n")
		sb.WriteString("#   dnf:\n")
		sb.WriteString("#     install: \"sudo dnf install -y {package}\"\n")
		sb.WriteString("#     uninstall: \"sudo dnf remove -y {package}\"\n")
		sb.WriteString("#     update: \"sudo dnf upgrade -y {package}\"\n")
	}

	sb.WriteString("\n# packages:\n")
	sb.WriteString("#   ripgrep:\n")
	sb.WriteString("#     binary: rg        # 'rg' is the executable; install arg is still 'ripgrep'\n")
	sb.WriteString("#     managers:\n")
	if len(selected) > 0 {
		for _, name := range selected {
			fmt.Fprintf(&sb, "#       %s:\n", name)
		}
		sb.WriteString("#       yum:\n")
		sb.WriteString("#         package: rg   # override name passed to this manager\n")
	} else {
		sb.WriteString("#       dnf:            # empty = use dnf defaults\n")
		sb.WriteString("#       yum:\n")
		sb.WriteString("#         package: rg   # override name passed to this manager\n")
	}

	return sb.String()
}
