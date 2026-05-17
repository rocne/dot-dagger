package adopter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/node"
	"github.com/rocne/dot-dagger/internal/pipeline"
)

// ConventionNames holds the active names for the three convention directories.
type ConventionNames struct {
	Shellrc string
	Bin     string
	Conf    string
}

// DefaultConventions returns the standard convention dir names.
func DefaultConventions() ConventionNames {
	return ConventionNames{Shellrc: "shellrc", Bin: "bin", Conf: "conf"}
}

// Inference is the result of inferring a dotfiles destination for a source file.
type Inference struct {
	DestRel string // relative path from dotfiles root, e.g. "conf/dot-bashrc"
	Reason  string // human-readable description of why this destination was chosen
	Unknown bool   // true if no rule matched — caller must require --to
}

// AdoptOptions configures an Adopt call.
type AdoptOptions struct {
	DotfilesRoot string
	Conventions  ConventionNames
	LinkRoot     string // resolved home/link-root dir (for symlink expansion)
	BinDir       string // resolved managed bin dir
	Force        bool
	DryRun       bool
}

// Infer returns the inferred dotfiles-relative destination for src.
// Inference rules (in priority order):
//  1. Executable bit → <bin>/<name>
//  2. Shell extension (.sh .bash .zsh .fish) → <shellrc>/<name>
//  3. Hidden file (.bashrc) → <conf>/dot-<name>
//  4. Config extension (.toml .yaml .conf …) → <conf>/<name>
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

	// Hidden dotfile (.bashrc, .gitconfig) → conf/dot-<name>
	if strings.HasPrefix(name, ".") && len(name) > 1 {
		dotName := "dot-" + name[1:]
		return Inference{
			DestRel: filepath.Join(conv.Conf, dotName),
			Reason:  "dotfile (dot- prefix added)",
		}
	}

	// Config-like extensions → conf/
	switch ext {
	case ".conf", ".config", ".toml", ".yaml", ".yml", ".ini", ".cfg", ".json":
		return Inference{
			DestRel: filepath.Join(conv.Conf, name),
			Reason:  "config file",
		}
	}

	return Inference{Unknown: true}
}

// Adopt copies src to <DotfilesRoot>/<destRel>, removes the original,
// and creates a symlink at the original src path (or the managed bin dir
// for bin/ files). Returns the pipeline.ActResult from pipeline.Act.
func Adopt(src, destRel string, opts AdoptOptions) (*pipeline.ActResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// copyFile copies src to dst, preserving permissions. dst must not exist.
func copyFile(src, dst string) error {
	return fmt.Errorf("not implemented")
}

// actionsFor returns the pipeline actions for a node at destRel,
// based on which convention dir it lives in.
func actionsFor(destRel, src string, opts AdoptOptions) []pipeline.Action {
	return nil
}

// suppress unused import errors until stubs are replaced
var _ = node.DeriveName  // used by Adopt (Task 4)
var _ = io.Copy          // used by copyFile (Task 4 Step 3)
