package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ActOptions configures Act behaviour.
type ActOptions struct {
	HomeDir string // replaces "~" in link destinations; defaults to os.UserHomeDir
	DryRun  bool   // validate without writing to filesystem
}

// Link represents a symlink to create.
type Link struct {
	Src  string // file to link to (absolute path of the dotfile)
	Dest string // symlink location (expanded)
}

// ActResult contains the outputs from Act.
type ActResult struct {
	Sourced []RawNode // nodes to be sourced, in order
	Links   []Link    // symlinks to create/verify
}

// Act executes actions for ordered nodes in the given slice.
// Nodes are processed in order; links are validated for conflicts before writing.
func Act(nodes []RawNode, opts ActOptions) (*ActResult, error) {
	home := opts.HomeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("act: resolve home dir: %w", err)
		}
	}

	res := &ActResult{}

	// First pass: collect all intended links and detect conflicts.
	destSeen := map[string]string{} // dest → logical name that claimed it
	for _, n := range nodes {
		noSource := false
		for _, a := range n.Actions {
			if a.Type == "no-source" {
				noSource = true
			}
		}

		for _, a := range n.Actions {
			switch a.Type {
			case "source":
				if !noSource {
					res.Sourced = append(res.Sourced, n)
				}
			case "link":
				dest := expandTilde(a.Dest, home)
				if prev, ok := destSeen[dest]; ok {
					return nil, fmt.Errorf("act: link conflict: %s and %s both link to %s", prev, n.LogicalName, dest)
				}
				destSeen[dest] = n.LogicalName
				res.Links = append(res.Links, Link{Src: n.Path, Dest: dest})
			}
		}
	}

	// Second pass: write links (skipped in dry run).
	if !opts.DryRun {
		for _, lnk := range res.Links {
			if err := createSymlink(lnk.Src, lnk.Dest); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func createSymlink(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("act: mkdir for %s: %w", dest, err)
	}
	// Remove existing symlink (but not a real file/dir).
	if fi, err := os.Lstat(dest); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dest)
		} else {
			return fmt.Errorf("act: %s exists and is not a symlink", dest)
		}
	}
	if err := os.Symlink(src, dest); err != nil {
		return fmt.Errorf("act: symlink %s → %s: %w", dest, src, err)
	}
	return nil
}
