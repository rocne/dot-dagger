package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
	"github.com/rocne/dot-dagger/internal/pipeline"
	"github.com/spf13/cobra"
)

func newBundleCmd(cfg *config) *cobra.Command {
	var (
		output     string
		includeEnv bool
	)

	cmd := &cobra.Command{
		Use:   "bundle <path>",
		Short: "Bundle a node and its transitive @after dependencies into a single script",
		Long: `Resolve a dotfiles node and all its transitive @after dependencies,
then concatenate them (dependencies first, in DAG order) into a single
portable shell script.

The target file is identified by absolute path or path relative to the
dotfiles repo root.

Examples:
  dotd bundle shellrc/my-script.sh
  dotd bundle shellrc/my-script.sh --output /tmp/bundled.sh
  dotd bundle shellrc/my-script.sh --include-env`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBundle(cmd, cfg, args[0], output, includeEnv)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write output to file (default: stdout)")
	cmd.Flags().BoolVar(&includeEnv, "include-env", false, "prepend resolved env as export lines")
	return cmd
}

func runBundle(cmd *cobra.Command, cfg *config, target, outputFile string, includeEnv bool) error {
	ordered, err := cfg.walkOrdered(cmd)
	if err != nil {
		return err
	}

	// Resolve target to absolute path.
	targetAbs := target
	if !filepath.IsAbs(target) {
		targetAbs = filepath.Join(cfg.files, target)
	}

	// Find the target node.
	targetIdx := -1
	for i, n := range ordered {
		if n.Path == targetAbs {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return &hintError{
			err:  fmt.Errorf("bundle: %q not found in active nodes", target),
			hint: "run `dotd list` to see active nodes — the path may be filtered out by its @when or lie outside the dotfiles root",
		}
	}

	// Collect transitive @after dependencies of the target.
	// A node is a dependency if it has a path before target in DAG order
	// AND the target (directly or transitively) depends on it.
	deps := collectDeps(ordered, targetIdx)

	// Build output.
	var sb strings.Builder

	sb.WriteString(fileutil.POSIXShebang + "\n")
	sb.WriteString(ecosystem.GeneratedFileHeader() + "\n\n")

	if includeEnv {
		resolved, resolveErr := resolveEnv(cfg)
		if resolveErr != nil {
			return annotateKeyError(resolveErr)
		}
		// Emit in sorted key order so the bundle is byte-for-byte reproducible
		// (map iteration is nondeterministic). Matches `dotd env show` ordering.
		for _, k := range slices.Sorted(maps.Keys(resolved)) {
			fmt.Fprintf(&sb, "export %s=%s\n", k, fileutil.ShellQuote(resolved[k]))
		}
		sb.WriteString("\n")
	}

	for _, n := range deps {
		content, err := os.ReadFile(n.Path)
		if err != nil {
			return fmt.Errorf("bundle: read %s: %w", n.Path, err)
		}
		fmt.Fprintf(&sb, "# --- %s ---\n", n.LogicalName)
		sb.Write(content)
		sb.WriteString("\n")
	}

	// Append the target itself.
	content, err := os.ReadFile(ordered[targetIdx].Path)
	if err != nil {
		return fmt.Errorf("bundle: read %s: %w", ordered[targetIdx].Path, err)
	}
	fmt.Fprintf(&sb, "# --- %s ---\n", ordered[targetIdx].LogicalName)
	sb.Write(content)

	out := sb.String()

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(out), fileutil.ModeFile); err != nil {
			return fmt.Errorf("bundle: write %s: %w", outputFile, err)
		}
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

// collectDeps returns all nodes in ordered[0..targetIdx-1] that the target
// transitively depends on via @after. Uses a reachability set built from
// the ordered list: since Kahn's algorithm produces dependency-before-dependent
// order, any node before target in the list that target @after-depends on
// (directly or transitively) is a dependency.
func collectDeps(ordered []pipeline.RawNode, targetIdx int) []pipeline.RawNode {
	if targetIdx == 0 {
		return nil
	}

	// Build name→index.
	nameIdx := map[string]int{}
	for i, n := range ordered {
		nameIdx[n.LogicalName] = i
	}

	// BFS from target backwards through @after edges.
	needed := map[int]bool{}
	queue := []int{targetIdx}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range ordered[cur].After {
			// Expand prefix refs.
			deps := pipeline.ResolveAfterRef(dep, ordered)
			for _, d := range deps {
				i, ok := nameIdx[d]
				if !ok || needed[i] {
					continue
				}
				needed[i] = true
				queue = append(queue, i)
			}
		}
	}

	var result []pipeline.RawNode
	for i := 0; i < targetIdx; i++ {
		if needed[i] {
			result = append(result, ordered[i])
		}
	}
	return result
}
