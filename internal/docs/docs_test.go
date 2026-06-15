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
