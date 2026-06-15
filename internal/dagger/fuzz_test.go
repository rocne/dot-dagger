package dagger

import (
	"strings"
	"testing"
)

// FuzzLoad asserts that decoding an arbitrary .dagger file never panics — it
// must return a node or an error, whatever the bytes. .dagger files are
// user-authored, so malformed/hostile content is expected input.
func FuzzLoad(f *testing.F) {
	seeds := []string{
		"",
		"when: os=linux\nactions: [link]\n",
		"link_root: \"~\"\ndefaults:\n  actions: [source]\n",
		"files:\n  base.sh:\n    name: core\n    after: [other]\n",
		"composition:\n  enabled: true\n",
		"compose: true\n",
		"conventions:\n  shellrc: rc\n  bin: b\n  config: c\n",
		"disable: true\n",
		"unknown_field: 1\n", // KnownFields(true) must reject, not crash
		":\n",                // malformed YAML
		"\t- not: valid",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		node, err := Load(strings.NewReader(input))
		if err != nil {
			return
		}
		// A successful decode must yield a usable node — exercise the accessor.
		_ = node.IsCompose()
	})
}
