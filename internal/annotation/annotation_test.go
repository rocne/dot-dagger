package annotation

import (
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Annotation
	}{
		{
			name:  "hash comment",
			input: "# @when os=macos",
			want:  []Annotation{{Key: "when", Value: "os=macos", Line: 1}},
		},
		{
			name:  "slash comment",
			input: "// @name foo",
			want:  []Annotation{{Key: "name", Value: "foo", Line: 1}},
		},
		{
			name:  "annotation with no value",
			input: "# @retain-prefix",
			want:  []Annotation{{Key: "retain-prefix", Value: "", Line: 1}},
		},
		{
			name:  "non-annotation comment ignored",
			input: "# just a comment",
			want:  nil,
		},
		{
			name:  "non-comment line ignored",
			input: "export FOO=bar",
			want:  nil,
		},
		{
			name: "multiple annotations",
			input: "# @when os=macos\n# @name myfile\n# @after base",
			want: []Annotation{
				{Key: "when", Value: "os=macos", Line: 1},
				{Key: "name", Value: "myfile", Line: 2},
				{Key: "after", Value: "base", Line: 3},
			},
		},
		{
			name: "annotations mixed with code",
			input: "#!/bin/bash\n# @when os=macos\nexport FOO=bar\n# @name foo",
			want: []Annotation{
				{Key: "when", Value: "os=macos", Line: 2},
				{Key: "name", Value: "foo", Line: 4},
			},
		},
		{
			name:  "leading whitespace stripped",
			input: "   # @when os=linux",
			want:  []Annotation{{Key: "when", Value: "os=linux", Line: 1}},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Scan(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Scan() len = %d, want %d\ngot:  %+v\nwant: %+v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Scan()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Value: "os=macos"},
		{Key: "name", Value: "foo"},
		{Key: "when", Value: "context=work"},
	}
	got := Get(anns, "when")
	if len(got) != 2 {
		t.Fatalf("Get() len = %d, want 2", len(got))
	}
	if got[0].Value != "os=macos" || got[1].Value != "context=work" {
		t.Errorf("Get() = %+v, unexpected values", got)
	}
}

func TestFirst(t *testing.T) {
	anns := []Annotation{
		{Key: "when", Value: "os=macos"},
		{Key: "name", Value: "foo"},
	}

	got, ok := First(anns, "name")
	if !ok {
		t.Fatal("First() ok = false, want true")
	}
	if got.Value != "foo" {
		t.Errorf("First().Value = %q, want %q", got.Value, "foo")
	}

	_, ok = First(anns, "after")
	if ok {
		t.Error("First() ok = true for missing key, want false")
	}
}

func TestCombineWhen(t *testing.T) {
	tests := []struct {
		name  string
		anns  []Annotation
		want  string
	}{
		{
			name: "single when",
			anns: []Annotation{{Key: "when", Value: "os=macos"}},
			want: "(os=macos)",
		},
		{
			name: "multiple when",
			anns: []Annotation{
				{Key: "when", Value: "os=macos OR os=linux"},
				{Key: "when", Value: "context=work"},
			},
			want: "(os=macos OR os=linux) AND (context=work)",
		},
		{
			name: "no when",
			anns: []Annotation{{Key: "name", Value: "foo"}},
			want: "",
		},
		{
			name: "empty when value ignored",
			anns: []Annotation{{Key: "when", Value: ""}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineWhen(tt.anns)
			if got != tt.want {
				t.Errorf("CombineWhen() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCoreKey(t *testing.T) {
	core := []string{"when", "name", "after", "symlink", "retain-prefix"}
	for _, k := range core {
		if !IsCoreKey(k) {
			t.Errorf("IsCoreKey(%q) = false, want true", k)
		}
	}
	nonCore := []string{"require", "request", "unknown", ""}
	for _, k := range nonCore {
		if IsCoreKey(k) {
			t.Errorf("IsCoreKey(%q) = true, want false", k)
		}
	}
}

func TestRegistryDispatch(t *testing.T) {
	t.Run("registered handler called", func(t *testing.T) {
		r := NewRegistry(Strict)
		called := false
		r.Register("require", func(value string, dryRun bool) error {
			called = true
			return nil
		})
		err := r.Dispatch(Annotation{Key: "require", Value: "nvim"}, false)
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if !called {
			t.Error("handler was not called")
		}
	})

	t.Run("dry-run passed to handler", func(t *testing.T) {
		r := NewRegistry(Strict)
		var gotDryRun bool
		r.Register("require", func(value string, dryRun bool) error {
			gotDryRun = dryRun
			return nil
		})
		_ = r.Dispatch(Annotation{Key: "require"}, true)
		if !gotDryRun {
			t.Error("dryRun not passed to handler")
		}
	})

	t.Run("strict mode: unknown key returns error", func(t *testing.T) {
		r := NewRegistry(Strict)
		err := r.Dispatch(Annotation{Key: "unknown", Line: 5}, false)
		if err == nil {
			t.Error("Dispatch() error = nil, want error for unknown key in strict mode")
		}
	})

	t.Run("warn mode: unknown key writes warning, no error", func(t *testing.T) {
		r := NewRegistry(Warn)
		var buf strings.Builder
		r.SetWarnOutput(&buf)
		err := r.Dispatch(Annotation{Key: "unknown", Line: 5}, false)
		if err != nil {
			t.Fatalf("Dispatch() error = %v, want nil in warn mode", err)
		}
		if !strings.Contains(buf.String(), "unknown") {
			t.Errorf("warning output %q does not mention key", buf.String())
		}
	})

	t.Run("core keys silently skipped", func(t *testing.T) {
		r := NewRegistry(Strict)
		for _, key := range []string{"when", "name", "after", "symlink", "retain-prefix"} {
			err := r.Dispatch(Annotation{Key: key}, false)
			if err != nil {
				t.Errorf("Dispatch(%q) error = %v, want nil for core key", key, err)
			}
		}
	})
}
