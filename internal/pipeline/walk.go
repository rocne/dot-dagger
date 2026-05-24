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
	Require       []string // hard package dependencies (@require / .dagger require:)
	Request       []string // soft package dependencies (@request / .dagger request:)
	LinkRoot      string   // from nearest ancestor .dagger with link_root set
	LinkRootDir   string   // abs path of dir where link_root was declared
	IsCompose     bool     // true if this file is a compose fragment
	ComposeTarget string   // abs path of compose target dir
}

// dirState is the accumulated defaults for a directory at walk time.
type dirState struct {
	when        string   // from defaults.when; ANDed with parent
	linkRoot    string   // from link_root; nearest non-empty wins
	linkRootDir string   // absolute path of dir where link_root was declared
	actions     []string // from defaults.actions; outermost accumulate
	isCompose   bool     // true if inside a composition-enabled dir
	composeDir  string   // absolute path of the compose target dir
}

// Walk traverses dotfilesRoot and returns a RawNode for every dotfile found.
// It skips .dagger files themselves. Disabled paths (via @disable or files: disable: true)
// are returned in the second slice instead of included in nodes.
func Walk(dotfilesRoot string) ([]RawNode, []string, error) {
	// Pre-scan .dagger files into a map.
	daggerMap := map[string]*dagger.ComposableNode{}

	if err := filepath.WalkDir(dotfilesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			cfg, err := dagger.LoadFile(filepath.Join(path, ".dagger"))
			if err != nil {
				return err
			}
			daggerMap[path] = cfg
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Build set of files declared in .dagger files: dicts.
	// These are skipped in the main walk and added separately below.
	filesWithDaggerEntry := map[string]bool{}
	for dirPath, cfg := range daggerMap {
		for fname := range cfg.Files {
			filesWithDaggerEntry[filepath.Join(dirPath, fname)] = true
		}
	}

	var nodes []RawNode
	var disabled []string

	// Walk to emit both compose-target directory nodes and file nodes.
	if err := filepath.WalkDir(dotfilesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path == dotfilesRoot {
				return nil
			}
			if filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			// Emit a compose-target node for directories with composition enabled.
			cfg, ok := daggerMap[path]
			if !ok || !cfg.IsCompose() {
				return nil
			}
			rel, relErr := filepath.Rel(dotfilesRoot, path)
			if relErr != nil {
				return relErr
			}
			// Compute parent-only cascade state (ancestors up to but not including this dir).
			parentDir := filepath.Dir(rel)
			parentRel := filepath.Join(parentDir, "x") // cascadeState uses Dir(relPath)
			state := cascadeState(dotfilesRoot, parentRel, daggerMap)

			// Dir-level actions: always starts with "compose", then user-declared actions.
			dirActions := append([]Action{{Type: "compose"}}, parseDaggerActions(cfg.Actions)...)

			// Dir-level when: cascade when AND dir's own when.
			effectiveWhen := combineWhen(state.when, cfg.When)

			// Dir's own link_root overrides inherited.
			linkRoot := state.linkRoot
			linkRootDir := state.linkRootDir
			if cfg.LinkRoot != "" {
				linkRoot = cfg.LinkRoot
				linkRootDir = path
			}

			// Apply .dagger name override if set; otherwise derive from path.
			logicalName := node.DeriveName(rel)
			if cfg.Name != "" {
				logicalName = cfg.Name
			}

			nodes = append(nodes, RawNode{
				Path:          path,
				LogicalName:   logicalName,
				EffectiveWhen: effectiveWhen,
				Actions:       dirActions,
				LinkRoot:      linkRoot,
				LinkRootDir:   linkRootDir,
				IsCompose:     false,
				ComposeTarget: path, // this node IS the compose target
			})
			return nil
		}

		base := filepath.Base(path)
		if base == ".dagger" || base == ".dotd.yaml" {
			return nil
		}
		// Files declared in a .dagger files: dict are handled below.
		if filesWithDaggerEntry[path] {
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
		anns = normalizeActionAnnotations(anns)

		// @disable: skip this file entirely.
		if _, ok := annotation.First(anns, "disable"); ok {
			disabled = append(disabled, path)
			return nil
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

		// Collect @require / @request package deps.
		var require, request []string
		for _, a := range annotation.Get(anns, "require") {
			if a.Args != "" {
				require = append(require, a.Args)
			}
		}
		for _, a := range annotation.Get(anns, "request") {
			if a.Args != "" {
				request = append(request, a.Args)
			}
		}

		// Apply @name override if present.
		logicalName := node.DeriveName(rel)
		if a, ok := annotation.First(anns, "name"); ok && a.Args != "" {
			logicalName = a.Args
		}

		n := RawNode{
			Path:          path,
			LogicalName:   logicalName,
			EffectiveWhen: effectiveWhen,
			Actions:       actions,
			After:         after,
			Require:       require,
			Request:       request,
			LinkRoot:      state.linkRoot,
			LinkRootDir:   state.linkRootDir,
			IsCompose:     state.isCompose,
			ComposeTarget: state.composeDir,
		}
		nodes = append(nodes, n)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Process .dagger files: dict entries.
	// These cover files that cannot carry annotations (binary, JSON, Lua, etc.).
	for dirPath, cfg := range daggerMap {
		for fname, fileNode := range cfg.Files {
			filePath := filepath.Join(dirPath, fname)
			if _, statErr := os.Stat(filePath); statErr != nil {
				continue // file doesn't exist, skip silently
			}

			// disable: true in .dagger files: dict skips the file.
			if fileNode.Disable {
				disabled = append(disabled, filePath)
				continue
			}

			rel, relErr := filepath.Rel(dotfilesRoot, filePath)
			if relErr != nil {
				return nil, nil, relErr
			}
			state := cascadeState(dotfilesRoot, rel, daggerMap)

			logicalName := node.DeriveName(rel)
			if fileNode.Name != "" {
				logicalName = fileNode.Name
			}

			fileWhen := ""
			if fileNode.When != "" {
				fileWhen = "(" + fileNode.When + ")"
			}
			effectiveWhen := combineWhen(state.when, fileWhen)

			var fileActions []Action
			seen := map[string]bool{}
			for _, actStr := range fileNode.Actions {
				act := parseActionString(actStr)
				if !seen[act.Type] {
					fileActions = append(fileActions, act)
					seen[act.Type] = true
				}
			}

			nodes = append(nodes, RawNode{
				Path:          filePath,
				LogicalName:   logicalName,
				EffectiveWhen: effectiveWhen,
				Actions:       fileActions,
				After:         fileNode.After,
				Require:       fileNode.Require,
				Request:       fileNode.Request,
				LinkRoot:      state.linkRoot,
			})
		}
	}

	return nodes, disabled, nil
}

// parseDaggerActions converts .dagger action strings (e.g. "link(~/.tmux.conf)") to Action structs.
func parseDaggerActions(strs []string) []Action {
	var actions []Action
	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if s == "compose" {
			actions = append(actions, Action{Type: "compose"})
		} else if s == "source" {
			actions = append(actions, Action{Type: "source"})
		} else if s == "no-source" {
			actions = append(actions, Action{Type: "no-source"})
		} else if strings.HasPrefix(s, "link(") && strings.HasSuffix(s, ")") {
			dest := s[5 : len(s)-1]
			actions = append(actions, Action{Type: "link", Dest: dest})
		}
	}
	return actions
}

// cascadeState computes the accumulated dirState for a file at relPath
// by walking ancestor directories from the root downward.
func cascadeState(root, relPath string, daggerMap map[string]*dagger.ComposableNode) dirState {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(relPath)), "/")

	state := dirState{}

	// Apply root .dagger defaults first.
	if cfg, ok := daggerMap[root]; ok {
		state = applyDefaults(state, cfg.Defaults, root)
		if cfg.LinkRoot != "" {
			state.linkRoot = cfg.LinkRoot
			state.linkRootDir = root
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
			state = applyDefaults(state, cfg.Defaults, cur)
			if cfg.LinkRoot != "" {
				state.linkRoot = cfg.LinkRoot
				state.linkRootDir = cur
			}
			// Detect compose target directories.
			if cfg.Composition.Enabled && !state.isCompose {
				state.isCompose = true
				state.composeDir = cur
			}
		}
	}

	return state
}

// applyDefaults merges a BasicNode's defaults into the accumulated state.
func applyDefaults(state dirState, defaults dagger.BasicNode, _ string) dirState {
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

// parseActionString converts an action string like "link(~/.gitconfig)" or "source"
// into an Action. Handles the "type(dest)" function-call syntax from .dagger files.
func parseActionString(s string) Action {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, '(')
	if i < 0 {
		return Action{Type: s}
	}
	typ := strings.TrimSpace(s[:i])
	rest := s[i+1:]
	j := strings.LastIndexByte(rest, ')')
	if j < 0 {
		return Action{Type: s} // malformed, treat whole string as type
	}
	return Action{Type: typ, Dest: strings.TrimSpace(rest[:j])}
}

// mergeActions produces the final Action list for a node.
// anns must be pre-normalized by normalizeActionAnnotations — only Key=="action" entries
// are processed; all other annotation keys are ignored.
func mergeActions(defaultActions []string, anns []annotation.Annotation) []Action {
	var actions []Action
	seen := map[string]bool{}
	// linkFromDefault tracks whether the only link action came from inherited defaults,
	// allowing a single file annotation to override the destination.
	linkFromDefault := false

	// Seed with inherited defaults.
	for _, actStr := range defaultActions {
		act := parseActionString(actStr)
		if act.Type != "" && !seen[act.Type] {
			actions = append(actions, act)
			seen[act.Type] = true
			if act.Type == "link" {
				linkFromDefault = true
			}
		}
	}

	// Apply normalized action annotations.
	for _, a := range anns {
		if a.Key != "action" {
			continue
		}
		act := parseActionString(a.Args)
		switch act.Type {
		case "compose":
			if !seen["compose"] {
				actions = append(actions, act)
				seen["compose"] = true
			}
		case "link":
			if !seen["link"] {
				actions = append(actions, act)
				seen["link"] = true
			} else if linkFromDefault {
				// Annotation overrides inherited default — replace dest in-place.
				for i := range actions {
					if actions[i].Type == "link" {
						actions[i].Dest = act.Dest
					}
				}
				linkFromDefault = false
			} else {
				// Second explicit annotation: keep both, validateNode reports the conflict.
				alreadyPresent := false
				for _, existing := range actions {
					if existing.Type == "link" && existing.Dest == act.Dest {
						alreadyPresent = true
						break
					}
				}
				if !alreadyPresent {
					actions = append(actions, act)
				}
			}
		case "source":
			if !seen["source"] && !seen["no-source"] {
				actions = append(actions, act)
				seen["source"] = true
			}
		case "no-source":
			// Remove any existing source, then record no-source.
			var filtered []Action
			for _, existing := range actions {
				if existing.Type != "source" {
					filtered = append(filtered, existing)
				}
			}
			actions = filtered
			delete(seen, "source")
			if !seen["no-source"] {
				actions = append(actions, act)
				seen["no-source"] = true
			}
		}
	}

	return actions
}
