package ui

import (
	"bytes"
	"strings"
	"testing"
)

// TestStatusHelpers_WriteToWriter smoke-tests the *f helpers — they should
// emit the substituted message to the writer without panicking.
func TestStatusHelpers_WriteToWriter(t *testing.T) {
	cases := []struct {
		name string
		fn   func(*bytes.Buffer)
		want string
	}{
		{"OKf", func(b *bytes.Buffer) { OKf(b, "all good %d", 1) }, "all good 1"},
		{"Warnf", func(b *bytes.Buffer) { Warnf(b, "careful") }, "careful"},
		{"Errf", func(b *bytes.Buffer) { Errf(b, "boom") }, "boom"},
		{"Tipf", func(b *bytes.Buffer) { Tipf(b, "try x") }, "try x"},
		{"Skipf", func(b *bytes.Buffer) { Skipf(b, "skipped") }, "skipped"},
		{"Headerf", func(b *bytes.Buffer) { Headerf(b, "title") }, "title"},
		{"Missingf", func(b *bytes.Buffer) { Missingf(b, "gone") }, "gone"},
		{"Wrongf", func(b *bytes.Buffer) { Wrongf(b, "broken") }, "broken"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.fn(&buf)
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("%s output %q missing %q", tc.name, buf.String(), tc.want)
			}
		})
	}
}

// TestColorHelpers_DoNotPanic exercises the small ANSI wrappers so a future
// edit can't accidentally panic on an empty string.
func TestColorHelpers_DoNotPanic(t *testing.T) {
	if got := OK("ok"); !strings.Contains(got, "ok") {
		t.Errorf("OK lost text: %q", got)
	}
	if got := Header(""); got != "" && !strings.Contains(got, "") {
		t.Errorf("Header(empty) = %q", got)
	}
	_ = Missing("x")
	_ = Wrong("x")
	_ = Conflict("x")
	_ = Installed("x")
	_ = Installable("x")
	_ = Skip("x")
	_ = Install("x")
	_ = HardMissing("x")
	_ = Arrow("→")
	_ = Key("k")
}
