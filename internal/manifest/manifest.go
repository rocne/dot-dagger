// Package manifest parses dotd-packages.yaml files and collects package requests.
package manifest

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rocne/dot-dagger/internal/packages"
	"github.com/rocne/dot-dagger/internal/predicate"
)

// Block is one predicate-scoped group of packages in a manifest file.
type Block struct {
	When     string   `yaml:"when"`
	Packages []string `yaml:"packages"`
}

// Parse reads a manifest YAML from r and returns its blocks.
func Parse(r io.Reader) ([]Block, error) {
	var blocks []Block
	if err := yaml.NewDecoder(r).Decode(&blocks); err != nil {
		return nil, fmt.Errorf("manifest: parse: %w", err)
	}
	return blocks, nil
}

// CollectFromPaths opens each path, parses it as a manifest, evaluates block
// predicates against env, and returns matching package requests.
// All packages from manifests are soft requests (not hard requirements).
func CollectFromPaths(paths []string, env map[string]string) ([]packages.PackageRequest, error) {
	ev := &predicate.Evaluator{Env: env}
	var reqs []packages.PackageRequest
	for _, path := range paths {
		r, err := collectFromPath(path, ev)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, r...)
	}
	return reqs, nil
}

func collectFromPath(path string, ev *predicate.Evaluator) ([]packages.PackageRequest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	blocks, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("manifest: %s: %w", path, err)
	}

	var reqs []packages.PackageRequest
	for _, b := range blocks {
		ok, err := evalWhen(ev, b.When)
		if err != nil {
			return nil, fmt.Errorf("manifest: %s: predicate %q: %w", path, b.When, err)
		}
		if !ok {
			continue
		}
		for _, pkg := range b.Packages {
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				reqs = append(reqs, packages.PackageRequest{
					Package:  pkg,
					Hard:     false,
					NodePath: path,
				})
			}
		}
	}
	return reqs, nil
}

func evalWhen(ev *predicate.Evaluator, when string) (bool, error) {
	if strings.TrimSpace(when) == "" {
		return true, nil
	}
	expr, err := predicate.Parse(when)
	if err != nil {
		return false, err
	}
	return ev.Eval(expr)
}
