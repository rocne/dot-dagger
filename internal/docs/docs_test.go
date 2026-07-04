package docs

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestRenderProse_OrderHeadersBodies(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":                 {Data: []byte("INTRO")},
		"docs/concepts/conditions.md":   {Data: []byte("COND")},
		"docs/reference/dotd.md":        {Data: []byte("REF")},
		"docs/getting-started/index.md": {Data: []byte("START")},
	}
	out, err := RenderProse(fsys)
	if err != nil {
		t.Fatal(err)
	}

	for _, h := range []string{
		"# === docs/index.md ===",
		"# === docs/getting-started/index.md ===",
		"# === docs/concepts/conditions.md ===",
		"# === docs/reference/dotd.md ===",
	} {
		if !strings.Contains(out, h) {
			t.Errorf("missing section header %q", h)
		}
	}

	// Exact curated order: index -> getting-started -> concepts -> reference.
	order := []string{
		"# === docs/index.md ===",
		"# === docs/getting-started/index.md ===",
		"# === docs/concepts/conditions.md ===",
		"# === docs/reference/dotd.md ===",
	}
	last := -1
	for _, h := range order {
		i := strings.Index(out, h)
		if i <= last {
			t.Errorf("section %q out of order (idx %d, prev %d)", h, i, last)
		}
		last = i
	}

	for _, body := range []string{"INTRO", "START", "COND", "REF"} {
		if !strings.Contains(out, body) {
			t.Errorf("missing doc body content: %q", body)
		}
	}
}

func TestTopics_SlugsTitlesOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":               {Data: []byte("# Welcome\n\nintro")},
		"docs/concepts/conditions.md": {Data: []byte("# Conditions\n\nbody")},
		"docs/reference/dotd.md":      {Data: []byte("no heading here")},
	}
	topics, err := Topics(fsys)
	if err != nil {
		t.Fatal(err)
	}

	want := []Topic{
		{Slug: "index", Path: "docs/index.md", Title: "Welcome"},
		{Slug: "concepts/conditions", Path: "docs/concepts/conditions.md", Title: "Conditions"},
		{Slug: "reference/dotd", Path: "docs/reference/dotd.md", Title: "reference/dotd"},
	}
	if len(topics) != len(want) {
		t.Fatalf("got %d topics, want %d: %+v", len(topics), len(want), topics)
	}
	for i, w := range want {
		if topics[i] != w {
			t.Errorf("topic[%d] = %+v, want %+v", i, topics[i], w)
		}
	}
}

func TestMatch_ExactSlugBeatsBasename(t *testing.T) {
	topics := []Topic{
		{Slug: "index"},
		{Slug: "getting-started/index"},
	}
	got := Match(topics, "index")
	if len(got) != 1 || got[0].Slug != "index" {
		t.Fatalf("Match(index) = %+v, want exact slug match only", got)
	}
}

func TestMatch_UniqueBasename(t *testing.T) {
	topics := []Topic{
		{Slug: "concepts/conditions"},
		{Slug: "reference/dotd"},
	}
	got := Match(topics, "conditions")
	if len(got) != 1 || got[0].Slug != "concepts/conditions" {
		t.Fatalf("Match(conditions) = %+v, want concepts/conditions", got)
	}
}

func TestMatch_AmbiguousBasenameReturnsAll(t *testing.T) {
	topics := []Topic{
		{Slug: "concepts/annotations"},
		{Slug: "reference/annotations"},
	}
	got := Match(topics, "annotations")
	if len(got) != 2 {
		t.Fatalf("Match(annotations) = %+v, want both candidates", got)
	}
}

func TestMatch_CaseInsensitiveAndNone(t *testing.T) {
	topics := []Topic{{Slug: "concepts/conditions"}}
	if got := Match(topics, "Conditions"); len(got) != 1 {
		t.Errorf("Match(Conditions) = %+v, want case-insensitive hit", got)
	}
	if got := Match(topics, "nope"); len(got) != 0 {
		t.Errorf("Match(nope) = %+v, want empty", got)
	}
}

func TestRenderProse_UnknownDirsAppendedAlphabetically(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":        {Data: []byte("I")},
		"docs/reference/a.md":  {Data: []byte("R")},
		"docs/guides/intro.md": {Data: []byte("G")},
		"docs/zzz/last.md":     {Data: []byte("Z")},
	}
	out, err := RenderProse(fsys)
	if err != nil {
		t.Fatal(err)
	}
	ri := strings.Index(out, "# === docs/reference/a.md ===")
	gi := strings.Index(out, "# === docs/guides/intro.md ===")
	zi := strings.Index(out, "# === docs/zzz/last.md ===")
	// Known (reference) before unknown; unknown dirs alphabetical (guides<zzz).
	if ri < 0 || ri >= gi || gi >= zi {
		t.Errorf("bad order: reference=%d guides=%d zzz=%d\n%s", ri, gi, zi, out)
	}
}
