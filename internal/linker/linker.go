// Package linker plans and applies symlinks for conf/ and bin/ nodes.
// Scripts are excluded — they are handled by initgen via init.sh sourcing.
//
// Destination derivation rules:
//
//	KindConf — path relative to the conf/ ancestor is transformed
//	  (nosync- stripped, dot- replaced with .) and joined under LinkRoot.
//	KindBin  — filename only, placed under BinDir.
//	@symlink annotation — explicit destination; absolute, ~/…, or relative to LinkRoot.
//
// Ownership: a symlink is owned by this tool if its current target is under RepoRoot.
// Foreign symlinks require --force to overwrite.
package linker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/rocne/dot-dagger/internal/ui"
)

// LinkState describes the current state of a symlink destination.
type LinkState int

const (
	// StateUnknown is the zero value before Check is called.
	StateUnknown LinkState = iota
	// StateOK — symlink exists and points to the correct source.
	StateOK
	// StateMissing — nothing at the expected destination.
	StateMissing
	// StateWrongTarget — symlink exists but points elsewhere.
	StateWrongTarget
	// StateConflict — real file (not a symlink) at the destination.
	StateConflict
)

func (s LinkState) String() string {
	switch s {
	case StateOK:
		return "OK"
	case StateMissing:
		return "Missing"
	case StateWrongTarget:
		return "WrongTarget"
	case StateConflict:
		return "Conflict"
	default:
		return "Unknown"
	}
}

// Link represents a planned symlink: Src → Dst.
type Link struct {
	// Src is the absolute path to the source file in the dotfiles repo.
	Src string
	// Dst is the absolute path where the symlink should be created.
	Dst string
	// State is populated by Check.
	State LinkState
	// Owned is true if the current symlink target is under RepoRoot.
	Owned bool
}

// Options configures the linker.
type Options struct {
	// RepoRoot is the root of the dotfiles repository. Used for ownership checks.
	RepoRoot string
	// LinkRoot is the default symlink destination root for conf/ files (default: $HOME).
	LinkRoot string
	// BinDir is the managed bin directory for bin/ files.
	BinDir string
}

// Plan computes the set of symlinks for the given active nodes.
// Nodes with KindScript are skipped (handled by initgen).
// Nodes with KindOther and no @symlink annotation are skipped.
func Plan(nodes []fileset.Node, opts Options) ([]Link, error) {
	if opts.LinkRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("linker: resolve home dir: %w", err)
		}
		opts.LinkRoot = home
	}
	if opts.BinDir == "" {
		opts.BinDir = filepath.Join(opts.LinkRoot, ".local", "bin", "dot-dagger")
	}

	var links []Link
	for _, n := range nodes {
		dst, err := destFor(n, opts)
		if err != nil {
			return nil, fmt.Errorf("linker: %s: %w", n.Path, err)
		}
		if dst == "" {
			continue // not linkable
		}
		links = append(links, Link{Src: n.Path, Dst: dst})
	}
	return links, nil
}

// Check populates the State and Owned fields for each link.
func Check(links []Link, repoRoot string) []Link {
	out := make([]Link, len(links))
	for i, l := range links {
		out[i] = l
		fi, err := os.Lstat(l.Dst)
		if os.IsNotExist(err) {
			out[i].State = StateMissing
			continue
		}
		if err != nil {
			out[i].State = StateMissing
			continue
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			// Real file or directory — conflict.
			out[i].State = StateConflict
			continue
		}

		target, err := os.Readlink(l.Dst)
		if err != nil {
			out[i].State = StateWrongTarget
			continue
		}

		if repoRoot != "" && strings.HasPrefix(target, repoRoot) {
			out[i].Owned = true
		}

		if target == l.Src {
			out[i].State = StateOK
		} else {
			out[i].State = StateWrongTarget
		}
	}
	return out
}

// Apply creates symlinks for all links in the list.
// Links that are already OK are skipped.
// If force is false, StateConflict and StateWrongTarget with Owned=false return an error.
func Apply(links []Link, force bool) error {
	for _, l := range links {
		if l.State == StateOK {
			continue
		}
		if l.State == StateConflict && !force {
			return fmt.Errorf("linker: %s: real file exists; use --force to overwrite", l.Dst)
		}
		if l.State == StateWrongTarget && !l.Owned && !force {
			return fmt.Errorf("linker: %s: foreign symlink exists; use --force to overwrite", l.Dst)
		}

		if err := os.MkdirAll(filepath.Dir(l.Dst), 0o755); err != nil {
			return fmt.Errorf("linker: mkdir %s: %w", filepath.Dir(l.Dst), err)
		}

		// Remove existing entry if present.
		if l.State != StateMissing {
			if err := os.Remove(l.Dst); err != nil {
				return fmt.Errorf("linker: remove %s: %w", l.Dst, err)
			}
		}

		if err := os.Symlink(l.Src, l.Dst); err != nil {
			return fmt.Errorf("linker: symlink %s → %s: %w", l.Src, l.Dst, err)
		}
	}
	return nil
}

// Remove deletes owned symlinks. Non-owned and missing links are skipped.
func Remove(links []Link) error {
	for _, l := range links {
		if !l.Owned || l.State == StateMissing {
			continue
		}
		if err := os.Remove(l.Dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("linker: remove %s: %w", l.Dst, err)
		}
	}
	return nil
}

// destFor returns the symlink destination for a node, or "" if not linkable.
func destFor(n fileset.Node, opts Options) (string, error) {
	// Resolve effective link root: per-node value (from .dot-dagger.yaml cascade) takes
	// precedence over the flat Options.LinkRoot.
	linkRoot := opts.LinkRoot
	if n.LinkRoot != "" {
		linkRoot = n.LinkRoot
	}

	// @symlink annotation takes precedence.
	if a, ok := annotation.First(n.Annotations, annotation.KeySymlink); ok && a.Value != "" {
		return resolveSymlinkDest(a.Value, linkRoot), nil
	}

	switch n.Kind {
	case fileset.KindConf:
		rel, err := confRelPath(n.Path)
		if err != nil {
			return "", err
		}
		return filepath.Join(linkRoot, rel), nil

	case fileset.KindBin:
		return filepath.Join(opts.BinDir, filepath.Base(n.Path)), nil

	default:
		return "", nil
	}
}

// resolveSymlinkDest resolves an @symlink destination according to the spec rules:
//
//	absolute  → used as-is
//	~/…       → relative to $HOME
//	else      → relative to linkRoot
func resolveSymlinkDest(dest, linkRoot string) string {
	switch {
	case filepath.IsAbs(dest):
		return dest
	case strings.HasPrefix(dest, "~/"):
		home, _ := os.UserHomeDir()
		return filepath.Join(home, dest[2:])
	default:
		return filepath.Join(linkRoot, dest)
	}
}

// confRelPath computes the destination-relative path for a conf/ file.
// Finds the conf/ ancestor in the absolute path, then transforms each component:
//   - strips nosync- prefix
//   - replaces dot- prefix with .
func confRelPath(absPath string) (string, error) {
	// Split into components and find the conf dir index.
	parts := strings.Split(filepath.ToSlash(absPath), "/")
	confIdx := -1
	for i, p := range parts {
		stripped := stripLinkerPrefixes(p)
		if stripped == "conf" {
			confIdx = i
			break
		}
	}
	if confIdx < 0 {
		return "", fmt.Errorf("no conf/ ancestor in path %s", absPath)
	}

	// Transform each component after conf/.
	rel := parts[confIdx+1:]
	result := make([]string, len(rel))
	for i, p := range rel {
		p = strings.TrimPrefix(p, "nosync-")
		p = strings.Replace(p, "dot-", ".", 1)
		result[i] = p
	}
	return filepath.Join(result...), nil
}

// stripLinkerPrefixes strips nosync- and dot- from a path component
// for the purpose of identifying special directory names.
func stripLinkerPrefixes(s string) string {
	s = strings.TrimPrefix(s, "nosync-")
	s = strings.TrimPrefix(s, "dot-")
	return s
}

// PrintCheckSummary writes per-link status rows and a summary line to w.
// If verbose is true, OK links are also shown.
func PrintCheckSummary(w io.Writer, links []Link, verbose bool) {
	var ok, missing, wrong, conflict int
	for _, l := range links {
		switch l.State {
		case StateOK:
			ok++
			if verbose {
				fmt.Fprintf(w, "  %-12s %s\n", ui.OK("ok"), l.Dst)
			}
		case StateMissing:
			missing++
			fmt.Fprintf(w, "  %-12s %s\n", ui.Missing("missing"), l.Dst)
		case StateWrongTarget:
			wrong++
			fmt.Fprintf(w, "  %-12s %s %s %s\n", ui.Wrong("wrong"), l.Dst, ui.Arrow("→"), l.Src)
		case StateConflict:
			conflict++
			fmt.Fprintf(w, "  %-12s %s\n", ui.Conflict("conflict"), l.Dst)
		}
	}
	fmt.Fprintf(w, "%s %d ok, %d missing, %d wrong-target, %d conflict\n",
		ui.Header("symlinks:"), ok, missing, wrong, conflict)
}

// PrintRemovePlan writes the dry-run preview of links that would be removed to w.
func PrintRemovePlan(w io.Writer, links []Link) {
	for _, l := range links {
		if l.Owned {
			fmt.Fprintf(w, "%s %s\n", ui.Wrong("remove"), l.Dst)
		}
	}
}

// CountOwned returns the number of owned links in the slice.
func CountOwned(links []Link) int {
	var n int
	for _, l := range links {
		if l.Owned {
			n++
		}
	}
	return n
}
