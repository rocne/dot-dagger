// Package docs assembles the embedded documentation into a single text blob.
// It is pure: it reads an fs.FS and returns a string, with no cobra or other
// command-layer dependency, so it is unit-testable against a fstest.MapFS.
package docs

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// docsRoot is the top-level directory the embedded FS is rooted under
// (//go:embed keeps the "docs/" path prefix).
const docsRoot = "docs"

// priority is the curated render order of top-level entries under docsRoot.
// Entries not listed here are appended after these, alphabetically — so a new
// doc section still ships (just unordered) rather than silently disappearing.
var priority = []string{"index.md", "getting-started", "concepts", "reference"}

// RenderProse concatenates the embedded markdown under docsRoot into one blob:
// a leading index of the sections that follow, then each file body prefixed
// with a "# === <repo-relative-path> ===" separator. The full path in the
// separator is load-bearing: the docs contain relative cross-links such as
// [x](../concepts/conditions.md) which don't resolve on stdout, but an agent
// can trace such a link to the matching section header in the same blob.
func RenderProse(fsys fs.FS) (string, error) {
	files, err := orderedFiles(fsys)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("# Documentation index\n\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteByte('\n')

	for _, f := range files {
		body, err := fs.ReadFile(fsys, f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "# === %s ===\n\n", f)
		b.Write(body)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// Topic is one embedded documentation page addressable from the CLI.
type Topic struct {
	Slug  string // page path under docsRoot without the .md suffix, e.g. "concepts/conditions"
	Path  string // full embedded path, e.g. "docs/concepts/conditions.md"
	Title string // first "# " heading in the page, or the slug when it has none
}

// Topics returns the embedded pages in the same curated order RenderProse
// uses, so listings and the --full blob never disagree about ordering.
func Topics(fsys fs.FS) ([]Topic, error) {
	files, err := orderedFiles(fsys)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, 0, len(files))
	for _, f := range files {
		body, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(f, docsRoot+"/"), ".md")
		title := slug
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		topics = append(topics, Topic{Slug: slug, Path: f, Title: title})
	}
	return topics, nil
}

// Match resolves query to candidate topics, case-insensitively. An exact
// slug match wins outright; otherwise every topic whose final path element
// equals the query is a candidate — the caller decides how to report
// ambiguity (len > 1) or no match (len == 0).
func Match(topics []Topic, query string) []Topic {
	q := strings.ToLower(query)
	for _, t := range topics {
		if strings.ToLower(t.Slug) == q {
			return []Topic{t}
		}
	}
	var out []Topic
	for _, t := range topics {
		if strings.ToLower(path.Base(t.Slug)) == q {
			out = append(out, t)
		}
	}
	return out
}

// orderedFiles returns the .md files under docsRoot in curated-then-alphabetical
// order (see priority). Files within a directory are alphabetical.
func orderedFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, docsRoot)
	if err != nil {
		return nil, err
	}

	rank := make(map[string]int, len(priority))
	for i, name := range priority {
		rank[name] = i
	}
	sort.SliceStable(entries, func(i, j int) bool {
		ri, oki := rank[entries[i].Name()]
		rj, okj := rank[entries[j].Name()]
		switch {
		case oki && okj:
			return ri < rj
		case oki:
			return true
		case okj:
			return false
		default:
			return entries[i].Name() < entries[j].Name()
		}
	})

	var files []string
	for _, e := range entries {
		name := e.Name()
		full := path.Join(docsRoot, name)
		if e.IsDir() {
			subs, err := mdFiles(fsys, full)
			if err != nil {
				return nil, err
			}
			files = append(files, subs...)
		} else if strings.HasSuffix(name, ".md") {
			files = append(files, full)
		}
	}
	return files, nil
}

// mdFiles walks dir and returns all .md files in lexical order. No explicit
// sort is needed: fs.WalkDir is documented to visit entries in lexical order.
func mdFiles(fsys fs.FS, dir string) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
