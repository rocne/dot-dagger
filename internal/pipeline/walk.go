// Package pipeline implements the v2 dotfiles pipeline: walk → filter → order → act.
package pipeline

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/dagger"
	"github.com/rocne/dot-dagger/internal/node"
)

// Action is a single action declared for a node.
type Action struct {
	Type string // "link", "source", "no-source", "compose"
	Dest string // non-empty only for "link"
}

// RawNode is a file discovered during Walk, before predicate evaluation.
type RawNode struct {
	Path          string // absolute filesystem path
	LogicalName   string // derived via node.DeriveName
	EffectiveWhen string // merged from ancestor defaults + file @when
	Actions       []Action
	After         []string // logical names (or prefix/) this node must come after
	LinkRoot      string   // from nearest ancestor .dagger with link_root set
	IsCompose     bool
	ComposeTarget string
}

// dirState is the accumulated defaults for a directory at walk time.
type dirState struct {
	when     string   // from defaults.when; ANDed with parent
	linkRoot string   // from link_root; nearest non-empty wins
	actions  []string // from defaults.actions; outermost accumulate
}

// Walk traverses dotfilesRoot and returns a RawNode for every dotfile found.
// It skips .dagger files themselves.
func Walk(dotfilesRoot string) ([]RawNode, error) {
	// Pre-scan .dagger files into a map.
	daggerMap := map[string]*dagger.ComposableNode{}

	if err := filepath.WalkDir(dotfilesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			cfg, err := dagger.LoadFile(filepath.Join(path, ".dagger"))
			if err != nil {
				return err
			}
			daggerMap[path] = cfg
		}
		return nil
	}); err != nil {
		return nil, err
	}

	var nodes []RawNode

	// cascadeStack tracks dirState per directory depth during walk.
	// We rebuild it per-file by walking ancestors from root.
	if err := filepath.WalkDir(dotfilesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == ".dagger" {
			return nil
		}

		rel, err := filepath.Rel(dotfilesRoot, path)
		if err != nil {
			return err
		}

		// Build cascaded state from root to parent dir.
		state := cascadeState(dotfilesRoot, rel, daggerMap)

		// Parse file annotations.
		anns, err := scanFileAnnotations(path)
		if err != nil {
			return err
		}

		// Merge effective when.
		fileWhen := annotation.CombineWhen(anns)
		effectiveWhen := combineWhen(state.when, fileWhen)

		// Compute actions: start from defaults, apply file overrides.
		actions := mergeActions(state.actions, anns)

		// Collect @after dependencies.
		var after []string
		for _, a := range annotation.Get(anns, "after") {
			if a.Args != "" {
				after = append(after, a.Args)
			}
		}

		n := RawNode{
			Path:          path,
			LogicalName:   node.DeriveName(rel),
			EffectiveWhen: effectiveWhen,
			Actions:       actions,
			After:         after,
			LinkRoot:      state.linkRoot,
		}
		nodes = append(nodes, n)
		return nil
	}); err != nil {
		return nil, err
	}

	return nodes, nil
}

// cascadeState computes the accumulated dirState for a file at relPath
// by walking ancestor directories from the root downward.
func cascadeState(root, relPath string, daggerMap map[string]*dagger.ComposableNode) dirState {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(relPath)), "/")

	state := dirState{}

	// Apply root .dagger defaults first.
	if cfg, ok := daggerMap[root]; ok {
		state = applyDefaults(state, cfg.Defaults)
		if cfg.LinkRoot != "" {
			state.linkRoot = cfg.LinkRoot
		}
	}

	// Walk each sub-directory component.
	cur := root
	for _, part := range parts {
		if part == "." || part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		if cfg, ok := daggerMap[cur]; ok {
			state = applyDefaults(state, cfg.Defaults)
			if cfg.LinkRoot != "" {
				state.linkRoot = cfg.LinkRoot
			}
		}
	}

	return state
}

// applyDefaults merges a BasicNode's defaults into the accumulated state.
func applyDefaults(state dirState, defaults dagger.BasicNode) dirState {
	if defaults.When != "" {
		state.when = combineWhen(state.when, "("+defaults.When+")")
	}
	if len(defaults.Actions) > 0 {
		state.actions = append(state.actions, defaults.Actions...)
	}
	return state
}

// combineWhen joins two when expressions with AND, wrapping each in parens if needed.
// Either may be empty.
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
		return a + " AND " + b
	}
}

// scanFileAnnotations opens path and returns its annotations.
func scanFileAnnotations(path string) ([]annotation.Annotation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return annotation.Scan(f)
}

// mergeActions produces the final Action list for a node.
// It starts from inherited default action types, then applies file-level annotations.
// File-level @link / @source / @no-source annotations add or replace.
func mergeActions(defaultActions []string, anns []annotation.Annotation) []Action {
	// Collect defaults into Action structs.
	var actions []Action
	seen := map[string]bool{}
	for _, typ := range defaultActions {
		if !seen[typ] {
			actions = append(actions, Action{Type: typ})
			seen[typ] = true
		}
	}

	// Apply file annotation overrides.
	for _, a := range anns {
		switch a.Key {
		case "link":
			if !seen["link"] {
				actions = append(actions, Action{Type: "link", Dest: a.Args})
				seen["link"] = true
			} else {
				// Replace existing link action dest.
				for i := range actions {
					if actions[i].Type == "link" {
						actions[i].Dest = a.Args
					}
				}
			}
		case "source":
			if !seen["source"] {
				actions = append(actions, Action{Type: "source"})
				seen["source"] = true
			}
		case "no-source":
			// no-source suppresses source.
			if !seen["no-source"] {
				actions = append(actions, Action{Type: "no-source"})
				seen["no-source"] = true
			}
		}
	}

	return actions
}
