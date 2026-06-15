package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/node"
)

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
		return nil, fmt.Errorf("act: HomeDir is required — set it to the resolved $HOME")
	}

	// Check for cross-node link conflicts up front (same normalization as Act).
	if err := CheckLinkConflicts(nodes, opts); err != nil {
		return nil, fmt.Errorf("act: %w", err)
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
		// Content is read even in dry-run: the dry-run plan must equal the
		// real plan (compose check runs Act in dry-run and compares Content
		// against the on-disk generated file). Dry-run skips writes only.
		data, err := os.ReadFile(n.Path)
		if err != nil {
			return nil, fmt.Errorf("act: read compose fragment %s: %w", n.Path, err)
		}
		composeFragments[n.ComposeTarget] = append(composeFragments[n.ComposeTarget], fragment{node: n, content: data})
	}

	// Main pass: process non-fragment nodes in order.
	genSeen := map[string]string{} // generated path → first compose target that claimed it
	for _, n := range nodes {
		// Skip compose fragments — already collected above.
		if n.IsCompose {
			continue
		}

		// Check for compose action on compose-target directory nodes.
		hasCompose := n.HasCompose()

		if hasCompose {
			// Assemble fragments for this compose target.
			frags := composeFragments[n.ComposeTarget]
			var assembled []byte
			for _, f := range frags {
				// Separate fragments with a newline so a fragment lacking a
				// trailing newline does not glue onto the next one's first
				// line. Only insert one when the preceding content does not
				// already end in "\n", to avoid doubling blank lines.
				if len(assembled) > 0 && assembled[len(assembled)-1] != '\n' {
					assembled = append(assembled, '\n')
				}
				assembled = append(assembled, f.content...)
			}

			// Determine generated file path.
			genPath := ""
			if opts.GeneratedDir != "" {
				genPath = filepath.Join(opts.GeneratedDir, ComposeFileName(n.Path))
				// Two compose targets whose names reduce to the same generated
				// file (e.g. "dot-foo.d" and "nosync-dot-foo.d") would silently
				// clobber each other's output; reject the collision instead.
				if prev, ok := genSeen[genPath]; ok {
					return nil, fmt.Errorf("act: compose targets %s and %s both generate %s", prev, n.Path, genPath)
				}
				genSeen[genPath] = n.Path
			}

			gen := Generated{Path: genPath, Content: assembled, Node: n}
			res.Generated = append(res.Generated, gen)

			// Apply remaining actions on the generated result (link, source).
			emitNodeActions(n, genPath, opts, res)
			continue
		}

		// Regular file node: apply actions.
		emitNodeActions(n, n.Path, opts, res)
	}

	// Second pass: write generated files and links (skipped in dry run).
	if !opts.DryRun {
		for _, gen := range res.Generated {
			if gen.Path == "" {
				continue
			}
			// Atomic like every other generated output — a sourced file must
			// never be observable half-written.
			if err := fileutil.WriteAtomic(gen.Path, gen.Content, fileutil.ModeFile); err != nil {
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

// emitNodeActions applies source and link actions for a node to res.
// srcPath is the effective source file path: the generated file for compose
// nodes, or n.Path for regular nodes. Conflict detection has already been run
// by CheckLinkConflicts; this function only emits.
func emitNodeActions(n RawNode, srcPath string, opts ActOptions, res *ActResult) {
	noSource := false
	for _, a := range n.Actions {
		if a.Type == ActionNoSource {
			noSource = true
		}
	}
	for _, a := range n.Actions {
		switch a.Type {
		case ActionCompose:
			// handled by the caller
		case ActionSource:
			if !noSource && srcPath != "" {
				if srcPath == n.Path {
					res.Sourced = append(res.Sourced, n)
				} else {
					// Compose node: synthetic node pointing to the generated file.
					synth := n
					synth.Path = srcPath
					res.Sourced = append(res.Sourced, synth)
				}
			}
		case ActionLink:
			dest := resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir)
			if srcPath != "" {
				res.Links = append(res.Links, Link{Src: srcPath, Dest: dest})
			}
		}
	}
}

// resolveLink computes the final link destination for a node.
// If dest is empty, it is derived from the node's link_root and filename.
func resolveLink(dest string, n RawNode, homeDir, binDir, configDir string) string {
	if dest == "" {
		dest = deriveLinkDest(n)
	}
	return expandDest(dest, homeDir, binDir, configDir)
}

// ComposeFileName derives the generated filename from a compose target dir path.
// Strips "nosync-" then "dot-" prefixes and the ".d" suffix.
// "nosync-dot-shellrc.d" → "shellrc", "dot-tmux.conf.d" → "tmux.conf"
func ComposeFileName(dirPath string) string {
	return strings.TrimSuffix(node.StripRepoPrefix(filepath.Base(dirPath)), ".d")
}

// deriveLinkDest computes a link destination from n.LinkRoot + relative filename.
// Applies nosync- strip and dot- → . transformation to every path component.
func deriveLinkDest(n RawNode) string {
	root := n.LinkRoot
	if root == "" || n.LinkRootDir == "" {
		return ""
	}
	rel, err := filepath.Rel(n.LinkRootDir, n.Path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, p := range parts {
		p = strings.TrimPrefix(p, "nosync-")
		if strings.HasPrefix(p, "dot-") {
			p = "." + p[4:]
		}
		parts[i] = p
	}
	return filepath.Join(root, filepath.Join(parts...))
}

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

func createSymlink(src, dest string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(dest), fileutil.ModeDir); err != nil {
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
