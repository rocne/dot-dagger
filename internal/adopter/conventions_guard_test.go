package adopter_test

import (
	"reflect"
	"testing"

	"github.com/rocne/dot-dagger/internal/adopter"
	"github.com/rocne/dot-dagger/internal/dagger"
)

// TestConventionConfigYAMLTagsMatchDirs guards that dagger.ConventionConfig
// yaml struct tags stay in sync with the adopter.Dir* convention dir-name
// constants. A one-sided rename of either fails this test.
func TestConventionConfigYAMLTagsMatchDirs(t *testing.T) {
	tests := []struct {
		field   string
		wantTag string
	}{
		{"Shellrc", adopter.DirShellrc},
		{"Bin", adopter.DirBin},
		{"Config", adopter.DirConfig},
	}
	rt := reflect.TypeOf(dagger.ConventionConfig{})
	for _, tc := range tests {
		f, ok := rt.FieldByName(tc.field)
		if !ok {
			t.Errorf("ConventionConfig: missing field %s", tc.field)
			continue
		}
		if got := f.Tag.Get("yaml"); got != tc.wantTag {
			t.Errorf("ConventionConfig.%s yaml tag: got %q, want %q", tc.field, got, tc.wantTag)
		}
	}
}
