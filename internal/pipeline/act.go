package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ActOptions configures Act behaviour.
type ActOptions struct {
	HomeDir      string // replaces "~" in link destinations; defaults to os.UserHomeDir
	BinDir       string // replaces "~bin" in link destinations
	GeneratedDir string // directory for compose-generated files
	DryRun       bool   // validate without writing to filesystem
	Force        bool   // overwrite non-symlink files on link conflicts
}

// Link represents a symlink to create.
type Link struct {
	Src  string // file to link to (absolute path of the dotfile)
	Dest string // symlink location (expanded)
}

// Generated holds the path and content of a compose-generated file.
type Generated struct {
	Path    string
	Content []byte
	Node    RawNode // the compose-target node that produced this file
}

// ActResult contains the outputs from Act.
type ActResult struct {
	Sourced   []RawNode   // nodes to be sourced, in order
	Links     []Link      // symlinks to create/verify
	Generated []Generated // compose-generated files
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

	// Group compose fragments by their ComposeTarget path.
	type fragment struct {
		node    RawNode
		content []byte
	}
	composeFragments := map[string][]fragment{} // composeDir → ordered fragments

	// Pre-pass: collect all compose fragments first (dirs are walked before contents).
	for _, n := range nodes {
		if !n.IsCompose {
			continue
		}
		var data []byte
		if !opts.DryRun {
			var err error
			data, err = os.ReadFile(n.Path)
			if err != nil {
				return nil, fmt.Errorf("act: read compose fragment %s: %w", n.Path, err)
			}
		}
		composeFragments[n.ComposeTarget] = append(composeFragments[n.ComposeTarget], fragment{node: n, content: data})
	}

	// Main pass: process non-fragment nodes in order.
	destSeen := map[string]string{} // dest → logical name that claimed it

	for _, n := range nodes {
		// Skip compose fragments — already collected above.
		if n.IsCompose {
			continue
		}

		// Check for compose action on compose-target directory nodes.
		hasCompose := false
		for _, a := range n.Actions {
			if a.Type == "compose" {
				hasCompose = true
				break
			}
		}

		if hasCompose {
			// Assemble fragments for this compose target.
			frags := composeFragments[n.ComposeTarget]
			var assembled []byte
			for _, f := range frags {
				assembled = append(assembled, f.content...)
			}

			// Determine generated file path.
			genPath := ""
			if opts.GeneratedDir != "" {
				// Derive filename from the compose dir basename:
				// "dot-shellrc-extras.sh.d" → "shellrc-extras.sh"
				dirBase := strings.TrimPrefix(filepath.Base(n.Path), "dot-")
				dirBase = strings.TrimSuffix(dirBase, ".d")
				genPath = filepath.Join(opts.GeneratedDir, dirBase)
			}

			gen := Generated{Path: genPath, Content: assembled, Node: n}
			res.Generated = append(res.Generated, gen)

			// Apply remaining actions on the generated result (link, source).
			noSource := false
			for _, a := range n.Actions {
				if a.Type == "no-source" {
					noSource = true
				}
			}
			for _, a := range n.Actions {
				switch a.Type {
				case "compose":
					// handled above
				case "source":
					if !noSource && genPath != "" {
						// Create a synthetic node for init.sh with the generated file path.
						synth := n
						synth.Path = genPath
						res.Sourced = append(res.Sourced, synth)
					}
				case "link":
					dest := resolveLink(a.Dest, n, home, opts.BinDir)
					if prev, ok := destSeen[dest]; ok {
						return nil, fmt.Errorf("act: link conflict: %s and %s both link to %s", prev, n.LogicalName, dest)
					}
					destSeen[dest] = n.LogicalName
					if genPath != "" {
						res.Links = append(res.Links, Link{Src: genPath, Dest: dest})
					}
				}
			}
			continue
		}

		// Regular file node: apply actions.
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
				dest := resolveLink(a.Dest, n, home, opts.BinDir)
				if prev, ok := destSeen[dest]; ok {
					return nil, fmt.Errorf("act: link conflict: %s and %s both link to %s", prev, n.LogicalName, dest)
				}
				destSeen[dest] = n.LogicalName
				res.Links = append(res.Links, Link{Src: n.Path, Dest: dest})
			}
		}
	}

	// Second pass: write generated files and links (skipped in dry run).
	if !opts.DryRun {
		for _, gen := range res.Generated {
			if gen.Path == "" {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(gen.Path), 0755); err != nil {
				return nil, fmt.Errorf("act: mkdir for generated %s: %w", gen.Path, err)
			}
			if err := os.WriteFile(gen.Path, gen.Content, 0644); err != nil {
				return nil, fmt.Errorf("act: write generated %s: %w", gen.Path, err)
			}
		}
		for _, lnk := range res.Links {
			if err := createSymlink(lnk.Src, lnk.Dest, opts.Force); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

// resolveLink computes the final link destination for a node.
// If dest is empty, it is derived from the node's link_root and filename.
func resolveLink(dest string, n RawNode, homeDir, binDir string) string {
	if dest == "" {
		dest = deriveLinkDest(n)
	}
	return expandDest(dest, homeDir, binDir)
}

// deriveLinkDest computes a link destination from n.LinkRoot + relative filename.
// Applies "dot-" → "." transformation to the first path component.
func deriveLinkDest(n RawNode) string {
	root := n.LinkRoot
	if root == "" || n.LinkRootDir == "" {
		return ""
	}
	rel, err := filepath.Rel(n.LinkRootDir, n.Path)
	if err != nil {
		return ""
	}
	// Apply dot- → . transformation to the first path component.
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if strings.HasPrefix(parts[0], "dot-") {
		parts[0] = "." + parts[0][4:]
	}
	return filepath.Join(root, strings.Join(parts, string(filepath.Separator)))
}

// expandDest expands "~/" and "~bin/" prefixes in a link destination.
func expandDest(path, homeDir, binDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	if binDir != "" {
		if path == "~bin" {
			return binDir
		}
		if strings.HasPrefix(path, "~bin/") {
			return filepath.Join(binDir, path[5:])
		}
	}
	return path
}


func createSymlink(src, dest string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("act: mkdir for %s: %w", dest, err)
	}
	// Handle existing path at dest.
	if fi, err := os.Lstat(dest); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dest)
		} else if force {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("act: remove existing %s: %w", dest, err)
			}
		} else {
			return fmt.Errorf("act: %s exists and is not a symlink", dest)
		}
	}
	if err := os.Symlink(src, dest); err != nil {
		return fmt.Errorf("act: symlink %s → %s: %w", dest, src, err)
	}
	return nil
}
