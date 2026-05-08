package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rocne/dot-dagger/internal/annotation"
	"github.com/rocne/dot-dagger/internal/dag"
	"github.com/rocne/dot-dagger/internal/fileset"
	"github.com/spf13/cobra"
)

func newBundleCmd(cfg *config) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "bundle <logical-name>",
		Short: "Bundle a node and its transitive @after dependencies into a single script",
		Long: `Resolve a dotfiles node and all its transitive @after dependencies,
then concatenate their contents into a single standalone shell script.

Useful for sharing a script that depends on your shell environment
without requiring the recipient to have dot-dagger installed.

Examples:
  dotd bundle shellrc.math
  dotd bundle shellrc.math --output shared.sh`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBundle(cmd, cfg, args[0], output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	return cmd
}

func runBundle(cmd *cobra.Command, cfg *config, target, output string) error {
	resolved, err := resolveEnv(cfg)
	if err != nil {
		return err
	}

	nodes, err := buildFileSet(cfg, resolved)
	if err != nil {
		return err
	}

	scripts := nodes.Scripts()
	ordered, err := dag.Build(scripts)
	if err != nil {
		return fmt.Errorf("dag: %w", err)
	}

	byName := make(map[string]fileset.Node, len(scripts))
	for _, n := range scripts {
		byName[n.LogicalName] = n
	}

	targetNode, ok := byName[target]
	if !ok {
		return fmt.Errorf("bundle: %q not found in active fileset", target)
	}

	deps := make(map[string]bool)
	bundleCollectDeps(targetNode, byName, scripts, deps)

	var bundle []fileset.Node
	for _, n := range ordered {
		if deps[n.LogicalName] || n.LogicalName == target {
			bundle = append(bundle, n)
		}
	}

	var b strings.Builder
	b.WriteString("#!/bin/sh\n# Bundled by dot-dagger — do not edit by hand.\n\n")
	for _, n := range bundle {
		content, err := os.ReadFile(n.Path)
		if err != nil {
			return fmt.Errorf("bundle: read %s: %w", n.Path, err)
		}
		fmt.Fprintf(&b, "# --- %s ---\n", n.LogicalName)
		b.Write(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	out := []byte(b.String())
	if output == "" {
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}
	return os.WriteFile(output, out, 0o644)
}

func bundleCollectDeps(n fileset.Node, byName map[string]fileset.Node, all []fileset.Node, deps map[string]bool) {
	for _, a := range annotation.Get(n.Annotations, "after") {
		for _, name := range bundleResolveAfter(a.Args, all) {
			if !deps[name] {
				deps[name] = true
				if dep, ok := byName[name]; ok {
					bundleCollectDeps(dep, byName, all, deps)
				}
			}
		}
	}
}

func bundleResolveAfter(ref string, nodes []fileset.Node) []string {
	if strings.HasSuffix(ref, "/") {
		prefix := strings.ReplaceAll(strings.TrimSuffix(ref, "/"), "/", ".") + "."
		var names []string
		for _, n := range nodes {
			if strings.HasPrefix(n.LogicalName, prefix) {
				names = append(names, n.LogicalName)
			}
		}
		return names
	}
	return []string{ref}
}
