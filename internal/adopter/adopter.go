package adopter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/node"
	"github.com/rocne/dot-dagger/internal/pipeline"
)

// DirShellrc is the canonical name for the shell-scripts convention directory.
const DirShellrc = "shellrc"

// DirBin is the canonical name for the bin-scripts convention directory.
const DirBin = "bin"

// DirConfig is the canonical name for the config-files convention directory.
const DirConfig = "config"

// ConventionNames holds the active names for the three convention directories.
type ConventionNames struct {
	Shellrc string
	Bin     string
	Config  string
}

// DefaultConventions returns the standard convention dir names.
func DefaultConventions() ConventionNames {
	return ConventionNames{Shellrc: DirShellrc, Bin: DirBin, Config: DirConfig}
}

// Inference is the result of inferring a dotfiles destination for a source file.
type Inference struct {
	DestRel string // relative path from dotfiles root, e.g. "config/dot-bashrc"
	Reason  string // human-readable description of why this destination was chosen
	Unknown bool   // true if no rule matched — caller must require --to
}

// AdoptOptions configures an Adopt call.
type AdoptOptions struct {
	DotfilesRoot string
	Conventions  ConventionNames
	HomeDir      string // resolved home dir (for symlink expansion)
	ConfigDir    string // resolved XDG config home dir
	BinDir       string // resolved managed bin dir
	Force        bool
	DryRun       bool
}

// Infer returns the inferred dotfiles-relative destination for src.
// Inference rules (in priority order):
//  1. Executable bit → <bin>/<name>
//  2. Shell extension (.sh .bash .zsh .fish) → <shellrc>/<name>
//  3. Hidden file (.bashrc) → <config>/dot-<name>
//  4. Config extension (.toml .yaml .conf …) → <config>/<name>
//  5. Unknown → Inference{Unknown: true}
func Infer(src string, info os.FileInfo, conv ConventionNames) Inference {
	name := info.Name()
	ext := strings.ToLower(filepath.Ext(name))

	// Executable bit → bin/
	if info.Mode()&0o111 != 0 {
		return Inference{
			DestRel: filepath.Join(conv.Bin, name),
			Reason:  "executable",
		}
	}

	// Shell script extensions → shellrc/
	switch ext {
	case ".sh", ".bash", ".zsh", ".fish":
		return Inference{
			DestRel: filepath.Join(conv.Shellrc, name),
			Reason:  "shell script",
		}
	}

	// Hidden dotfile (.bashrc, .gitconfig) → config/dot-<name>
	if strings.HasPrefix(name, ".") && len(name) > 1 {
		dotName := node.PrefixDot + name[1:]
		return Inference{
			DestRel: filepath.Join(conv.Config, dotName),
			Reason:  "dotfile (dot- prefix added)",
		}
	}

	// Config-like extensions → config/
	switch ext {
	case ".conf", ".config", ".toml", ".yaml", ".yml", ".ini", ".cfg", ".json":
		return Inference{
			DestRel: filepath.Join(conv.Config, name),
			Reason:  "config file",
		}
	}

	return Inference{Unknown: true}
}

// Adopt copies src to <DotfilesRoot>/<destRel>, removes the original,
// and creates a symlink at the original src path (or the managed bin dir
// for bin/ files). Returns the pipeline.ActResult from pipeline.Act.
func Adopt(src, destRel string, opts AdoptOptions) (*pipeline.ActResult, error) {
	destAbs := filepath.Join(opts.DotfilesRoot, destRel)

	// Fail fast if dest already exists.
	if _, err := os.Stat(destAbs); err == nil {
		return nil, fmt.Errorf("adopt: destination already exists: %s", destAbs)
	}

	rel, err := filepath.Rel(opts.DotfilesRoot, destAbs)
	if err != nil {
		return nil, fmt.Errorf("adopt: rel path: %w", err)
	}

	actions := actionsFor(destRel, src, opts)
	n := pipeline.NewFileNode(destAbs, node.DeriveName(rel), actions)
	actOpts := pipeline.ActOptions{
		HomeDir:   opts.HomeDir,
		ConfigDir: opts.ConfigDir,
		BinDir:    opts.BinDir,
		DryRun:    opts.DryRun,
		Force:     opts.Force,
	}

	// In dry-run, skip all filesystem writes.
	if opts.DryRun {
		return pipeline.Act([]pipeline.RawNode{n}, actOpts)
	}

	if err := copyFile(src, destAbs); err != nil {
		return nil, err
	}

	if err := os.Remove(src); err != nil {
		// Copy succeeded but remove failed — both copies exist, safe state.
		return nil, fmt.Errorf("adopt: remove original %s: %w", src, err)
	}

	res, err := pipeline.Act([]pipeline.RawNode{n}, actOpts)
	if err != nil {
		// Remove succeeded, Act failed — file moved but no symlink.
		// Attempt to restore the original.
		if renameErr := os.Rename(destAbs, src); renameErr == nil {
			return nil, fmt.Errorf("adopt: create symlink: %w (original restored at %s)", err, src)
		}
		return nil, fmt.Errorf("adopt: create symlink: %w — recovery: mv %s %s", err, destAbs, src)
	}
	return res, nil
}

// actionsFor returns pipeline actions for the node at destRel.
// - shellrc/: source action (no symlink)
// - bin/:     explicit link to binDir/<name>
// - config/ or other: explicit link back to src (original path)
func actionsFor(destRel, src string, opts AdoptOptions) []pipeline.Action {
	parts := strings.SplitN(filepath.ToSlash(destRel), "/", 2)
	first := parts[0]
	name := filepath.Base(destRel)
	conv := opts.Conventions

	switch first {
	case conv.Shellrc:
		return []pipeline.Action{{Type: pipeline.ActionSource}}
	case conv.Bin:
		return []pipeline.Action{{Type: pipeline.ActionLink, Dest: filepath.Join(opts.BinDir, name)}}
	default:
		// config/ and any --to path: symlink at original src location.
		return []pipeline.Action{{Type: pipeline.ActionLink, Dest: src}}
	}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), fileutil.ModeDir); err != nil {
		return fmt.Errorf("adopt: mkdir %s: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("adopt: open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	srcInfo, err := in.Stat()
	if err != nil {
		return fmt.Errorf("adopt: stat %s: %w", src, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("adopt: create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("adopt: copy to %s: %w", dst, err)
	}
	return nil
}
