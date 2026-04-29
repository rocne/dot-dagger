// Package composer assembles compose targets: directories whose files are
// concatenated into a single generated file. The generated file is then
// handled by the linker or initgen exactly like any other file of its kind.
//
// Pipeline position: env → fileset → packages → compose → links → init.sh
package composer

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/daggeryaml"
	"github.com/rocne/dot-dagger/internal/fileset"
)

// invalidFragmentAnnotations lists annotation keys that are errors on fragments.
var invalidFragmentAnnotations = []string{
	annotation.KeySymlink,
	annotation.KeySource,
	annotation.KeyNoSource,
	annotation.KeyRequire,
	annotation.KeyRequest,
	annotation.KeyRetainPrefix,
}

// State describes the drift status of a compose target.
type State int

const (
	StateOK      State = iota
	StateMissing       // generated file does not exist
	StateStale         // generated file exists but content differs from current fragments
)

func (s State) String() string {
	switch s {
	case StateOK:
		return "ok"
	case StateMissing:
		return "missing"
	case StateStale:
		return "stale"
	default:
		return "unknown"
	}
}

// Target holds a resolved compose target and its active fragment nodes.
type Target struct {
	Dir        string       // absolute path to compose target directory
	Kind       fileset.Kind // convention kind: KindScript, KindConf, or KindBin
	OutputName string       // derived output logical name
	Fragments  []fileset.Node
	linkRoot   string // effective link_root for KindConf @symlink computation
	symlinkDest string // absolute symlink destination for KindConf targets; empty for others
}

// TargetStatus is the result of a Check call for one compose target.
type TargetStatus struct {
	Target     Target
	OutputPath string
	State      State
}

// TargetSummary is used by List.
type TargetSummary struct {
	Dir        string
	OutputName string
	Kind       fileset.Kind
	Fragments  []fileset.Node
}

// Options configures the compose stage.
type Options struct {
	// GeneratedDir is the directory where assembled files are written.
	// Default: ~/.local/share/dot-dagger/generated
	GeneratedDir string
	// LinkRoot is the default symlink root for conf/ targets whose fragments
	// carry no explicit link_root. Mirrors linker.Options.LinkRoot so that
	// the baked @symlink annotation in the synthetic node points to the same
	// base as the linker would use. When empty, falls back to os.UserHomeDir().
	LinkRoot string
	// DryRun skips file writes.
	DryRun bool
}

// Apply runs the compose stage for all KindCompose fragment nodes:
//  1. Group fragments by compose target directory.
//  2. Validate fragment annotations.
//  3. Order fragments via a per-target sub-DAG.
//  4. Concatenate and write (atomically) to GeneratedDir.
//  5. Return synthetic fileset.Node values for each active target,
//     carrying the correct Kind (KindScript/KindConf/KindBin).
//
// Targets with no active fragments are skipped entirely.
func Apply(fragments []fileset.Node, opts Options) ([]fileset.Node, error) {
	targets, err := groupTargets(fragments, opts.GeneratedDir, opts.LinkRoot)
	if err != nil {
		return nil, err
	}

	var result []fileset.Node
	for _, t := range targets {
		outPath := filepath.Join(opts.GeneratedDir, t.OutputName)

		ordered, err := dag.Build(t.Fragments)
		if err != nil {
			return nil, fmt.Errorf("composer: DAG for %s: %w", t.Dir, err)
		}

		content, err := concatenate(ordered)
		if err != nil {
			return nil, fmt.Errorf("composer: read fragments for %s: %w", t.Dir, err)
		}

		if !opts.DryRun {
			if err := writeAtomic(outPath, content); err != nil {
				return nil, err
			}
			if t.Kind == fileset.KindBin {
				if err := os.Chmod(outPath, 0o755); err != nil {
					return nil, fmt.Errorf("composer: chmod %s: %w", outPath, err)
				}
			}
		}

		result = append(result, syntheticNode(t, outPath))
	}
	return result, nil
}

// Check reports the drift state for each compose target without writing anything.
func Check(fragments []fileset.Node, opts Options) ([]TargetStatus, error) {
	targets, err := groupTargets(fragments, opts.GeneratedDir, opts.LinkRoot)
	if err != nil {
		return nil, err
	}

	var statuses []TargetStatus
	for _, t := range targets {
		outPath := filepath.Join(opts.GeneratedDir, t.OutputName)
		st := TargetStatus{Target: t, OutputPath: outPath}

		ordered, err := dag.Build(t.Fragments)
		if err != nil {
			return nil, fmt.Errorf("composer: DAG for %s: %w", t.Dir, err)
		}

		want, err := concatenate(ordered)
		if err != nil {
			return nil, fmt.Errorf("composer: read fragments for %s: %w", t.Dir, err)
		}

		got, err := os.ReadFile(outPath)
		if os.IsNotExist(err) {
			st.State = StateMissing
		} else if err != nil {
			st.State = StateMissing
		} else if sha256.Sum256(got) != sha256.Sum256(want) {
			st.State = StateStale
		} else {
			st.State = StateOK
		}

		statuses = append(statuses, st)
	}
	return statuses, nil
}

// List returns a summary of compose targets without executing them.
func List(fragments []fileset.Node, generatedDir string) ([]TargetSummary, error) {
	targets, err := groupTargets(fragments, generatedDir, "")
	if err != nil {
		return nil, err
	}
	summaries := make([]TargetSummary, len(targets))
	for i, t := range targets {
		summaries[i] = TargetSummary{
			Dir:        t.Dir,
			OutputName: t.OutputName,
			Kind:       t.Kind,
			Fragments:  t.Fragments,
		}
	}
	return summaries, nil
}

// groupTargets groups KindCompose nodes by ComposeTarget directory, resolves output
// names, validates fragment annotations, and checks for duplicate output names.
func groupTargets(fragments []fileset.Node, generatedDir, defaultLinkRoot string) ([]Target, error) {
	// Build ordered target list (preserve deterministic order by first-seen target).
	var order []string
	groups := make(map[string][]fileset.Node)
	for _, f := range fragments {
		if f.Kind != fileset.KindCompose {
			continue
		}
		if _, seen := groups[f.ComposeTarget]; !seen {
			order = append(order, f.ComposeTarget)
		}
		groups[f.ComposeTarget] = append(groups[f.ComposeTarget], f)
	}

	var errs []error
	var targets []Target
	seenOutputNames := make(map[string]string) // output name → target dir

	for _, dir := range order {
		frags := groups[dir]
		if len(frags) == 0 {
			continue
		}

		// Validate fragment annotations.
		if err := validateFragments(frags); err != nil {
			errs = append(errs, err)
			continue
		}

		// Load .dotd.yaml for name override and link_root.
		cfg, err := daggeryaml.LoadFile(filepath.Join(dir, ".dotd.yaml"))
		if err != nil {
			errs = append(errs, fmt.Errorf("composer: load %s/.dotd.yaml: %w", dir, err))
			continue
		}

		kind := frags[0].ComposeTargetKind
		linkRoot := frags[0].LinkRoot

		outputName, err := deriveOutputName(dir, cfg.Dotd.Name)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if prev, conflict := seenOutputNames[outputName]; conflict {
			errs = append(errs, fmt.Errorf("composer: output name %q conflict: %s and %s", outputName, prev, dir))
			continue
		}
		seenOutputNames[outputName] = dir

		t := Target{
			Dir:        dir,
			Kind:       kind,
			OutputName: outputName,
			Fragments:  frags,
			linkRoot:   linkRoot,
		}
		if kind == fileset.KindConf {
			effectiveLinkRoot := linkRoot
			if effectiveLinkRoot == "" {
				effectiveLinkRoot = defaultLinkRoot
			}
			t.symlinkDest = confDest(dir, effectiveLinkRoot, cfg.Dotd.Name)
		}
		targets = append(targets, t)
	}

	return targets, errors.Join(errs...)
}

// deriveOutputName computes the output logical name for a compose target directory.
// If dotdName is non-empty it is used directly (raw, no transforms).
// Otherwise: strip nosync-, strip dot-, strip .d suffix from the directory basename.
func deriveOutputName(dir, dotdName string) (string, error) {
	if dotdName != "" {
		return dotdName, nil
	}
	base := filepath.Base(dir)
	base = strings.TrimPrefix(base, "nosync-")
	base = strings.TrimPrefix(base, "dot-")
	if !strings.HasSuffix(base, ".d") {
		// Warn only — not a hard error per spec.
		fmt.Fprintf(os.Stderr, "composer: warning: compose target %s has no .d suffix\n", dir)
	}
	base = strings.TrimSuffix(base, ".d")
	if base == "" {
		return "", fmt.Errorf("composer: empty output name derived from %s", dir)
	}
	return base, nil
}

// validateFragments checks that no fragment carries an annotation that is invalid
// in a compose context (@symlink, @source, @no-source, @require, @request, @retain-prefix).
func validateFragments(frags []fileset.Node) error {
	var errs []error
	for _, f := range frags {
		for _, key := range invalidFragmentAnnotations {
			if _, ok := annotation.First(f.Annotations, key); ok {
				errs = append(errs, fmt.Errorf("composer: fragment %s: @%s is invalid inside a compose target", f.Path, key))
			}
		}
	}
	return errors.Join(errs...)
}

// concatenate reads the fragments in order and joins their contents.
func concatenate(nodes []fileset.Node) ([]byte, error) {
	var buf bytes.Buffer
	for _, n := range nodes {
		data, err := os.ReadFile(n.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", n.Path, err)
		}
		buf.Write(data)
		// Ensure each fragment ends with a newline so they don't run together.
		if len(data) > 0 && data[len(data)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

// writeAtomic writes content to path via a temp file + rename (same pattern as initgen).
func writeAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("composer: mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".compose-*.tmp")
	if err != nil {
		return fmt.Errorf("composer: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("composer: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("composer: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("composer: rename to %s: %w", path, err)
	}
	return nil
}

// syntheticNode builds a fileset.Node for the generated file. For KindConf targets,
// a @symlink annotation is added with the pre-computed absolute destination so that
// the linker does not need to scan for a conf/ ancestor in the generated path.
func syntheticNode(t Target, outPath string) fileset.Node {
	n := fileset.Node{
		Kind:        t.Kind,
		Path:        outPath,
		LogicalName: t.OutputName,
		LinkRoot:    t.linkRoot,
	}

	if t.Kind == fileset.KindConf && t.symlinkDest != "" {
		n.Annotations = []annotation.Annotation{
			{Key: annotation.KeySymlink, Value: t.symlinkDest},
		}
	}

	return n
}

// confDest computes the absolute symlink destination for a KindConf compose target.
//
// If dotdName is non-empty (dotd.name override), the destination filename is derived
// from dotdName by applying the dot-→. transform. Otherwise it is derived from the
// target dir basename: strip nosync-, strip .d, apply dot-→. transform.
//
// Intermediate path components between conf/ and the target dir basename are
// transformed the same way as the linker's confRelPath: strip nosync-, dot-→. .
func confDest(targetDir, linkRoot, dotdName string) string {
	if linkRoot == "" {
		home, _ := os.UserHomeDir()
		linkRoot = home
	}

	parts := strings.Split(filepath.ToSlash(targetDir), "/")
	confIdx := -1
	for i, p := range parts {
		s := strings.TrimPrefix(p, "nosync-")
		s = strings.TrimPrefix(s, "dot-")
		if s == "conf" {
			confIdx = i
			break
		}
	}

	// Destination filename component.
	var destFilename string
	if dotdName != "" {
		// dotd.name override: apply dot-→. transform directly.
		destFilename = applyDotTransform(dotdName)
	} else {
		targetBasename := parts[len(parts)-1]
		destFilename = strings.TrimPrefix(targetBasename, "nosync-")
		destFilename = strings.TrimSuffix(destFilename, ".d")
		destFilename = applyDotTransform(destFilename)
	}

	if confIdx < 0 {
		// No conf/ ancestor — join directly under linkRoot.
		return filepath.Join(linkRoot, destFilename)
	}

	// Intermediate components (between conf/ and target dir basename).
	intermediate := parts[confIdx+1 : len(parts)-1]
	result := make([]string, 0, len(intermediate)+1)
	for _, p := range intermediate {
		p = strings.TrimPrefix(p, "nosync-")
		p = strings.Replace(p, "dot-", ".", 1)
		result = append(result, p)
	}
	result = append(result, destFilename)
	return filepath.Join(append([]string{linkRoot}, result...)...)
}

// applyDotTransform replaces a leading "dot-" with "." in a filename component.
func applyDotTransform(s string) string {
	if strings.HasPrefix(s, "dot-") {
		return "." + s[4:]
	}
	return s
}
