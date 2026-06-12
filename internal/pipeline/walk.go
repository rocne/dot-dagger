// Package pipeline implements the v2 dotfiles pipeline: walk → filter → order → act.
package pipeline

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/dagger"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/node"
)

// Action type constants for Action.Type.
// NOTE: the user-facing option list in internal/annotation/registry.go
// (ActionType.Options) must stay in sync with these values; annotation
// cannot import pipeline (import cycle).
const (
	ActionCompose  = "compose"
	ActionSource   = "source"
	ActionNoSource = "no-source"
	ActionLink     = "link"
)


// Action is a single action declared for a node.
type Action struct {
	Type string // ActionLink, ActionSource, ActionNoSource, ActionCompose
	Dest string // non-empty only for ActionLink
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

// NewFileNode constructs a RawNode suitable for single-file processing by Act
// (e.g., the adopt command, where Walk is not used). Sets the minimum fields
// Act requires: Path, LogicalName, Actions. EffectiveWhen is empty so the node
// is always active; LinkRoot/IsCompose are zero-valued.
func NewFileNode(path, logicalName string, actions []Action) RawNode {
	return RawNode{
		Path:        path,
		LogicalName: logicalName,
		Actions:     actions,
	}
}

// HasCompose reports whether n has the compose action.
func (n RawNode) HasCompose() bool {
	for _, a := range n.Actions {
		if a.Type == ActionCompose {
			return true
		}
	}
	return false
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
			cfg, err := dagger.LoadFile(filepath.Join(path, ecosystem.ConfigFile))
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

			// Dir-level actions: always starts with ActionCompose, then user-declared actions.
			dirActions := append([]Action{{Type: ActionCompose}}, parseDaggerActions(cfg.Actions)...)

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
		if base == ecosystem.ConfigFile || base == ecosystem.LegacyConfigFile {
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
		if _, ok := annotation.First(anns, annotation.KeyDisable); ok {
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
		for _, a := range annotation.Get(anns, annotation.KeyAfter) {
			if a.Args != "" {
				after = append(after, a.Args)
			}
		}

		// Collect @require / @request package deps.
		var require, request []string
		for _, a := range annotation.Get(anns, annotation.KeyRequire) {
			if a.Args != "" {
				require = append(require, a.Args)
			}
		}
		for _, a := range annotation.Get(anns, annotation.KeyRequest) {
			if a.Args != "" {
				request = append(request, a.Args)
			}
		}

		// Apply @name override if present.
		logicalName := node.DeriveName(rel)
		if a, ok := annotation.First(anns, annotation.KeyName); ok && a.Args != "" {
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
// It delegates to parseActionString so dir-level actions: accepts the same arbitrary type(dest)
// syntax as file-level annotations, including unknown types that pass through harmlessly.
func parseDaggerActions(strs []string) []Action {
	var actions []Action
	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		actions = append(actions, parseActionString(s))
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
	head, body, _ := annotation.SplitParen(s)
	return Action{Type: head, Dest: body}
}

// mergeActions produces the final Action list for a node.
//
// Precondition: anns must be pre-normalized by normalizeActionAnnotations
// (only Key=="action" entries are processed; all other annotation keys are
// ignored). All known callers within Walk run that normalization first; if
// a future caller is added outside Walk, it must normalize before calling.
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
			if act.Type == ActionLink {
				linkFromDefault = true
			}
		}
	}

	// Apply normalized action annotations.
	for _, a := range anns {
		if a.Key != annotation.KeyAction {
			continue
		}
		act := parseActionString(a.Args)
		switch act.Type {
		case ActionCompose:
			if !seen[ActionCompose] {
				actions = append(actions, act)
				seen[ActionCompose] = true
			}
		case ActionLink:
			if !seen[ActionLink] {
				actions = append(actions, act)
				seen[ActionLink] = true
			} else if linkFromDefault {
				// Annotation overrides inherited default — replace dest in-place.
				for i := range actions {
					if actions[i].Type == ActionLink {
						actions[i].Dest = act.Dest
					}
				}
				linkFromDefault = false
			} else {
				// Second explicit annotation: keep both, validateNode reports the conflict.
				alreadyPresent := false
				for _, existing := range actions {
					if existing.Type == ActionLink && existing.Dest == act.Dest {
						alreadyPresent = true
						break
					}
				}
				if !alreadyPresent {
					actions = append(actions, act)
				}
			}
		case ActionSource:
			if !seen[ActionSource] && !seen[ActionNoSource] {
				actions = append(actions, act)
				seen[ActionSource] = true
			}
		case ActionNoSource:
			// Remove any existing source, then record no-source.
			var filtered []Action
			for _, existing := range actions {
				if existing.Type != ActionSource {
					filtered = append(filtered, existing)
				}
			}
			actions = filtered
			delete(seen, ActionSource)
			if !seen[ActionNoSource] {
				actions = append(actions, act)
				seen[ActionNoSource] = true
			}
		}
	}

	return actions
}
