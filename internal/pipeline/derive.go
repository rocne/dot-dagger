package pipeline

import (
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/dagger"
	"github.com/rocne/dot-dagger/internal/ecosystem"
)

// DerivedLinkDest reports the fully-expanded symlink destination that a
// subsequent Walk+Act would produce for the file at fileAbs under
// dotfilesRoot, or "" when no link action would apply to it. It mirrors the
// file-node path of Walk: ancestor .dagger cascade (link_root + default
// actions), a files: dict entry in the containing directory's .dagger if one
// exists, and otherwise the file's own annotations, then resolves the link
// destination exactly as Act would with opts' anchors.
//
// contentPath is the file whose annotations are scanned — normally fileAbs
// itself, but a caller planning a move (adopt --dry-run) can point it at the
// not-yet-moved source file.
func DerivedLinkDest(dotfilesRoot, fileAbs, contentPath string, opts ActOptions) (string, error) {
	rel, err := filepath.Rel(dotfilesRoot, fileAbs)
	if err != nil {
		return "", err
	}

	daggerMap, err := loadAncestorDaggers(dotfilesRoot, rel)
	if err != nil {
		return "", err
	}
	state := cascadeState(dotfilesRoot, rel, daggerMap)

	var actions []Action
	parentCfg := daggerMap[filepath.Dir(fileAbs)]
	if fileNode, ok := filesEntry(parentCfg, filepath.Base(fileAbs)); ok {
		// files: dict entry — Walk uses its actions verbatim (no defaults merge).
		seen := map[string]bool{}
		for _, actStr := range fileNode.Actions {
			act := parseActionString(actStr)
			if !seen[act.Type] {
				actions = append(actions, act)
				seen[act.Type] = true
			}
		}
	} else {
		anns, err := scanFileAnnotations(contentPath)
		if err != nil {
			return "", err
		}
		anns = normalizeActionAnnotations(anns)
		actions = mergeActions(state.actions, anns)
	}

	n := RawNode{Path: fileAbs, LinkRoot: state.linkRoot, LinkRootDir: state.linkRootDir}
	for _, a := range actions {
		if a.Type == ActionLink {
			return resolveLink(a.Dest, n, opts.HomeDir, opts.BinDir, opts.ConfigDir), nil
		}
	}
	return "", nil
}

// loadAncestorDaggers loads the .dagger files on the ancestor path from root
// down to Dir(rel), keyed by absolute directory path — the exact subset of
// Walk's daggerMap that cascadeState needs for a file at rel.
func loadAncestorDaggers(root, rel string) (map[string]*dagger.ComposableNode, error) {
	m := map[string]*dagger.ComposableNode{}
	cfg, err := dagger.LoadFile(filepath.Join(root, ecosystem.ConfigFile))
	if err != nil {
		return nil, err
	}
	m[root] = cfg

	cur := root
	for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
		if part == "." || part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		cfg, err := dagger.LoadFile(filepath.Join(cur, ecosystem.ConfigFile))
		if err != nil {
			return nil, err
		}
		m[cur] = cfg
	}
	return m, nil
}

// filesEntry looks up name in cfg's files: dict, tolerating a nil cfg.
func filesEntry(cfg *dagger.ComposableNode, name string) (dagger.NamedNode, bool) {
	if cfg == nil {
		return dagger.NamedNode{}, false
	}
	n, ok := cfg.Files[name]
	return n, ok
}
