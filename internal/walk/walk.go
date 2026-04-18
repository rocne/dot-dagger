// Package walk traverses a dotfiles directory tree and produces a flat list
// of file nodes, each carrying its annotations and effective @when expression.
package walk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/daggeryaml"
	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// expandTilde replaces a leading ~/ with the user's home directory.
func expandTilde(s string) (string, error) {
	if strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("walk: expand tilde: %w", err)
		}
		return filepath.Join(home, s[2:]), nil
	}
	return s, nil
}

// Kind identifies which special directory type a file belongs to.
type Kind int

const (
	KindOther  Kind = iota
	KindScript      // under shellrc/
	KindConf        // under conf/
	KindBin         // under bin/
)

func (k Kind) String() string {
	switch k {
	case KindScript:
		return "script"
	case KindConf:
		return "conf"
	case KindBin:
		return "bin"
	default:
		return "other"
	}
}

// Node represents a single file discovered during a walk.
type Node struct {
	// Path is the absolute filesystem path to the file.
	Path string

	// Kind identifies the special directory type this file belongs to.
	Kind Kind

	// LogicalName is the DAG identity: nosync-/dot- stripped per component,
	// extension stripped from last component, joined with ".".
	LogicalName string

	// Annotations are the raw annotations parsed from the file header.
	Annotations []annotation.Annotation

	// EffectiveWhen is the combined @when expression for this file.
	// It merges cascading directory defaults with the file's own @when.
	// Empty string means unconditionally active.
	EffectiveWhen string

	// LinkRoot is the symlink destination root for this file, derived from
	// the nearest ancestor .dotd.yaml with link.link_root set.
	// Empty means use the linker's default (Options.LinkRoot).
	LinkRoot string
}

// Default convention directory names. Override via dotd.conventions in root .dotd.yaml.
const (
	DirShellrc = "shellrc"
	DirConf    = "conf"
	DirBin     = "bin"
)

// Walk traverses the dotfiles repo at root and returns all file nodes.
// It respects .dotd.yaml config at each directory level.
// Convention dirs (shellrc/, conf/, bin/ by default) are recognised anywhere unless
// already inside a convention dir — at which point they are treated as regular dirs.
// Convention dir names can be overridden via dotd.conventions in the root .dotd.yaml.
func Walk(root string) ([]Node, error) {
	root = filepath.Clean(root)
	rootCfg, err := daggeryaml.LoadFile(filepath.Join(root, ecosystem.ConfigFile))
	if err != nil {
		return nil, err
	}
	dirNames := buildDirNames(rootCfg.Dotd.Conventions)
	var nodes []Node
	err = walkDir(root, root, KindOther, false, "", "", dirNames, &nodes)
	return nodes, err
}

// buildDirNames constructs the special-dir → Kind map, applying any convention overrides.
func buildDirNames(c daggeryaml.ConventionsSection) map[string]Kind {
	shellrc := c.Shellrc
	if shellrc == "" {
		shellrc = DirShellrc
	}
	conf := c.Conf
	if conf == "" {
		conf = DirConf
	}
	bin := c.Bin
	if bin == "" {
		bin = DirBin
	}
	return map[string]Kind{
		shellrc: KindScript,
		conf:    KindConf,
		bin:     KindBin,
	}
}

// walkDir recurses into dir.
//
//   - root:             repo root (for logical name computation)
//   - dir:              current directory being walked
//   - inheritedKind:    kind inherited from a parent convention dir (KindOther = not in one)
//   - inSpecialDir:     true if we are already inside a convention dir
//   - cascadeWhen:      accumulated @when expression from ancestor .dotd.yaml defaults
//   - cascadeLinkRoot:  link_root from nearest ancestor .dotd.yaml link section
//   - dirNames:         map of directory base name → Kind for this walk
func walkDir(root, dir string, inheritedKind Kind, inSpecialDir bool, cascadeWhen, cascadeLinkRoot string, dirNames map[string]Kind, nodes *[]Node) error {
	// Load .dotd.yaml for this directory.
	cfg, err := daggeryaml.LoadFile(filepath.Join(dir, ecosystem.ConfigFile))
	if err != nil {
		return err
	}

	// Gate traversal: if directory.when is set and doesn't match, skip entirely.
	// (Predicate evaluation happens in fileset; here we only track the expression.)
	// We pass the directory-level when upward — if it's set, stop traversal for
	// callers that evaluate predicates lazily. For the walker we always descend
	// and let fileset filter. But we do combine cascading defaults.
	dirDefaultWhen := combineWhen(cascadeWhen, cfg.Dotd.Defaults.When)

	// link_root cascade: inner .dotd.yaml overrides outer. Expand ~/ at walk time.
	effectiveLinkRoot := cascadeLinkRoot
	if cfg.Link.LinkRoot != "" {
		expanded, err := expandTilde(cfg.Link.LinkRoot)
		if err != nil {
			return err
		}
		effectiveLinkRoot = expanded
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Build per-file override map from .dotd.yaml files section.
	fileOverrides := make(map[string]*daggeryaml.FileEntry)
	for i := range cfg.Dotd.Files {
		fe := &cfg.Dotd.Files[i]
		fileOverrides[fe.Path] = fe
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ecosystem.ConfigFile {
			continue // config file is metadata, not a managed node
		}
		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			// Determine kind for this subdirectory.
			childKind := inheritedKind
			childInSpecial := inSpecialDir

			if !inSpecialDir {
				if k, ok := dirNames[stripPrefixes(name)]; ok {
					childKind = k
					childInSpecial = true
				}
			}

			childCascade := dirDefaultWhen
			if err := walkDir(root, fullPath, childKind, childInSpecial, childCascade, effectiveLinkRoot, dirNames, nodes); err != nil {
				return err
			}
			continue
		}

		// File node.
		anns, err := readAnnotations(fullPath)
		if err != nil {
			// Unreadable file: skip silently (could be binary, etc.)
			continue
		}

		// Apply .dotd.yaml file overrides for non-annotatable files.
		if override, ok := fileOverrides[name]; ok {
			anns = applyFileEntryOverrides(anns, override)
		}

		fileWhen := fileWhenExpr(anns)
		effectiveWhen := combineWhen(dirDefaultWhen, fileWhen)

		_, retainPrefix := annotation.First(anns, annotation.KeyRetainPrefix)
		logicalName := logicalNameFor(root, fullPath, retainPrefix)
		if nameAnn, ok := annotation.First(anns, annotation.KeyName); ok && nameAnn.Value != "" {
			logicalName = nameAnn.Value
		}

		*nodes = append(*nodes, Node{
			Path:          fullPath,
			Kind:          inheritedKind,
			LogicalName:   logicalName,
			Annotations:   anns,
			EffectiveWhen: effectiveWhen,
			LinkRoot:      effectiveLinkRoot,
		})
	}
	return nil
}

// readAnnotations opens a file and scans its header for annotations.
func readAnnotations(path string) ([]annotation.Annotation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return annotation.ScanHeader(f)
}

// applyFileEntryOverrides synthesises annotations from a .dotd.yaml FileEntry,
// overwriting any conflicting annotations already present.
func applyFileEntryOverrides(anns []annotation.Annotation, fe *daggeryaml.FileEntry) []annotation.Annotation {
	// Remove existing entries for keys we are about to set.
	keysToSet := map[string]string{}
	if fe.When != "" {
		keysToSet[annotation.KeyWhen] = fe.When
	}
	if fe.After != "" {
		keysToSet[annotation.KeyAfter] = fe.After
	}
	if fe.Name != "" {
		keysToSet[annotation.KeyName] = fe.Name
	}
	if fe.Symlink != "" {
		keysToSet[annotation.KeySymlink] = fe.Symlink
	}

	var result []annotation.Annotation
	for _, a := range anns {
		if _, overriding := keysToSet[a.Key]; !overriding {
			result = append(result, a)
		}
	}
	for k, v := range keysToSet {
		result = append(result, annotation.Annotation{Key: k, Value: v})
	}
	if fe.RetainPrefix {
		result = append(result, annotation.Annotation{Key: annotation.KeyRetainPrefix})
	}
	if fe.Disable {
		result = append(result, annotation.Annotation{Key: annotation.KeyDisable})
	}
	if fe.NoSource {
		result = append(result, annotation.Annotation{Key: annotation.KeyNoSource})
	}
	if fe.Source {
		result = append(result, annotation.Annotation{Key: annotation.KeySource})
	}
	return result
}

// logicalNameFor computes the logical name of path relative to root.
// Per-component: strip nosync-, strip dot-. Final component: also strip extension.
// If retainPrefix is true, neither dot- nor nosync- is stripped from the final component
// (extension is still stripped).
func logicalNameFor(root, path string, retainPrefix bool) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	result := make([]string, 0, len(parts))
	for i, p := range parts {
		last := i == len(parts)-1
		if last && retainPrefix {
			// keep dot- and nosync- as-is on the filename
		} else {
			p = stripPrefixes(p)
		}
		if last {
			p = stripExt(p)
		}
		if p != "" {
			result = append(result, p)
		}
	}
	return strings.Join(result, ".")
}

// stripPrefixes removes leading nosync- and dot- from a path component.
func stripPrefixes(s string) string {
	s = strings.TrimPrefix(s, "nosync-")
	s = strings.TrimPrefix(s, "dot-")
	return s
}

// stripExt removes the file extension from a filename.
func stripExt(s string) string {
	if ext := filepath.Ext(s); ext != "" {
		return s[:len(s)-len(ext)]
	}
	return s
}

// fileWhenExpr extracts @when values from annotations and joins them with AND.
// Single values are returned as-is (no wrapping parens).
// Multiple values are each wrapped in parens for correct precedence.
func fileWhenExpr(anns []annotation.Annotation) string {
	whens := annotation.Get(anns, annotation.KeyWhen)
	var vals []string
	for _, a := range whens {
		if a.Value != "" {
			vals = append(vals, a.Value)
		}
	}
	switch len(vals) {
	case 0:
		return ""
	case 1:
		return vals[0]
	default:
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = "(" + v + ")"
		}
		return strings.Join(parts, " AND ")
	}
}

// combineWhen joins two @when expressions with AND, wrapping each in parens.
// Returns empty string if both are empty.
func combineWhen(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	default:
		return "(" + a + ") AND (" + b + ")"
	}
}
