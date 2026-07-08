package adopter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/dagger"
	"github.com/rocne/dot-dagger/internal/ecosystem"
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

// PersistResult describes whether adopt needed to record the link destination
// in the destination directory's .dagger for apply to round-trip, and how the
// recording went. A future apply re-derives the symlink destination from the
// .dagger cascade; when that derivation would differ from the symlink adopt
// just made, the original location must be persisted as a files: entry or
// apply creates a second, wrong symlink (issue #191).
type PersistResult struct {
	Needed     bool   // derived dest differs from adopt's symlink — an entry must be recorded
	Persisted  bool   // the files: entry was written to DaggerPath
	Dest       string // home-contracted destination recorded (or to record)
	Derived    string // what apply would derive without the entry ("" = apply would not link at all)
	DaggerPath string // .dagger file that was (or must be) updated
	Snippet    string // exact YAML to add manually when Needed && !Persisted
	Err        error  // why automatic persistence failed (when Needed && !Persisted)
}

// Adopt copies src to <DotfilesRoot>/<destRel>, removes the original,
// and creates a symlink at the original src path (or the managed bin dir
// for bin/ files). When the destination a future apply would derive for the
// adopted file differs from that symlink, Adopt also records the destination
// as a files: entry in the containing directory's .dagger (see PersistResult;
// a failure to record never fails the adopt — it is reported for manual
// follow-up instead). Returns the pipeline.ActResult from pipeline.Act.
func Adopt(src, destRel string, opts AdoptOptions) (*pipeline.ActResult, *PersistResult, error) {
	destAbs := filepath.Join(opts.DotfilesRoot, destRel)

	// Fail fast if dest already exists.
	if _, err := os.Stat(destAbs); err == nil {
		return nil, nil, fmt.Errorf("adopt: destination already exists: %s", destAbs)
	}

	rel, err := filepath.Rel(opts.DotfilesRoot, destAbs)
	if err != nil {
		return nil, nil, fmt.Errorf("adopt: rel path: %w", err)
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
		res, err := pipeline.Act([]pipeline.RawNode{n}, actOpts)
		if err != nil {
			return nil, nil, err
		}
		// The file has not moved — scan annotations from src.
		return res, planPersist(src, destRel, src, opts), nil
	}

	if err := copyFile(src, destAbs); err != nil {
		return nil, nil, err
	}

	if err := os.Remove(src); err != nil {
		// Copy succeeded but remove failed — both copies exist, safe state.
		return nil, nil, fmt.Errorf("adopt: remove original %s: %w", src, err)
	}

	res, err := pipeline.Act([]pipeline.RawNode{n}, actOpts)
	if err != nil {
		// Remove succeeded, Act failed — file moved but no symlink.
		// Attempt to restore the original.
		if renameErr := os.Rename(destAbs, src); renameErr == nil {
			return nil, nil, fmt.Errorf("adopt: create symlink: %w (original restored at %s)", err, src)
		}
		return nil, nil, fmt.Errorf("adopt: create symlink: %w — recovery: mv %s %s", err, destAbs, src)
	}

	persist := planPersist(src, destRel, destAbs, opts)
	if persist.Needed {
		if perr := dagger.SetFileLink(persist.DaggerPath, filepath.Base(destRel), persist.Dest); perr != nil {
			// The adopt itself succeeded (file moved, symlink made) — never
			// fail it over bookkeeping. Report for manual follow-up.
			persist.Err = perr
		} else {
			persist.Persisted = true
		}
	}
	return res, persist, nil
}

// PlanPersist computes, without writing anything, whether adopting src at
// destRel would require recording a link destination (and where). For use by
// dry-run callers; src must still exist at its original location.
func PlanPersist(src, destRel string, opts AdoptOptions) *PersistResult {
	return planPersist(src, destRel, src, opts)
}

// planPersist decides whether the symlink adopt creates for destRel survives
// a future apply. contentPath is the current location of the file's content:
// src before the move (dry-run), destAbs after.
func planPersist(src, destRel, contentPath string, opts AdoptOptions) *PersistResult {
	destAbs := filepath.Join(opts.DotfilesRoot, destRel)
	res := &PersistResult{
		DaggerPath: filepath.Join(filepath.Dir(destAbs), ecosystem.ConfigFile),
	}

	// Which symlink does adopt itself create?
	var adoptDest string
	for _, a := range actionsFor(destRel, src, opts) {
		if a.Type == pipeline.ActionLink {
			adoptDest = a.Dest
		}
	}
	if adoptDest == "" {
		return res // shellrc/: sourced, no symlink to round-trip
	}

	derived, err := pipeline.DerivedLinkDest(opts.DotfilesRoot, destAbs, contentPath, pipeline.ActOptions{
		HomeDir:   opts.HomeDir,
		BinDir:    opts.BinDir,
		ConfigDir: opts.ConfigDir,
	})
	if err != nil {
		// The file (or an ancestor .dagger) can't be read the way apply's
		// walk would read it — apply would fail the same way, and a files:
		// entry sidesteps the in-file scan. Treat as "apply derives nothing".
		derived = ""
	}
	res.Derived = derived
	if derived != "" && filepath.Clean(derived) == filepath.Clean(adoptDest) {
		return res // apply already derives the same destination
	}

	res.Needed = true
	res.Dest = fileutil.ContractHome(adoptDest, opts.HomeDir)
	res.Snippet = dagger.FileLinkSnippet(filepath.Base(destRel), res.Dest)
	return res
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
