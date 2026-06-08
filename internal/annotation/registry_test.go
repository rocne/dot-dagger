package annotation

import (
	"strings"
	"testing"
)

func TestFormat_When(t *testing.T) {
	assertEqual(t, WhenType{}.Format("os=macos"), "# @when(os=macos)")
}

func TestFormat_After(t *testing.T) {
	assertEqual(t, AfterType{}.Format("shellrc.base"), "# @after(shellrc.base)")
}

func TestFormat_Require(t *testing.T) {
	assertEqual(t, RequireType{}.Format("git"), "# @require(git)")
}

func TestFormat_Request(t *testing.T) {
	assertEqual(t, RequestType{}.Format("ripgrep"), "# @request(ripgrep)")
}

func TestFormat_Action(t *testing.T) {
	assertEqual(t, ActionType{}.Format("source"), "# @action(source)")
}

func TestFormat_Name(t *testing.T) {
	assertEqual(t, NameType{}.Format("shellrc.aliases"), "# @name(shellrc.aliases)")
}

func TestFormat_Disable(t *testing.T) {
	assertEqual(t, DisableType{}.Format(""), "# @disable")
}

func TestValidate_WhenAcceptsKeyValue(t *testing.T) {
	err := WhenType{}.Validate("os=macos")
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidate_WhenAcceptsMultiValue(t *testing.T) {
	err := WhenType{}.Validate("os=linux,macos")
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidate_WhenRejectsMissingEquals(t *testing.T) {
	err := WhenType{}.Validate("invalid")
	if err == nil {
		t.Error("want error for missing =, got nil")
	}
}

func TestValidate_WhenRejectsEmptyKey(t *testing.T) {
	err := WhenType{}.Validate("=value")
	if err == nil {
		t.Error("want error for empty key, got nil")
	}
}

func TestValidate_WhenRejectsEmptyValue(t *testing.T) {
	err := WhenType{}.Validate("key=")
	if err == nil {
		t.Error("want error for empty value, got nil")
	}
}

func TestValidate_OtherTypesAlwaysNil(t *testing.T) {
	types := []AnnotationType{AfterType{}, RequireType{}, RequestType{}, ActionType{}, NameType{}}
	for _, tt := range types {
		if err := tt.Validate("anything"); err != nil {
			t.Errorf("%T.Validate() = %v, want nil", tt, err)
		}
	}
}

func TestRegistry_AllKeysPresent(t *testing.T) {
	want := []string{"when", "after", "require", "request", "action", "name", "disable"}
	if len(Registry) != len(want) {
		t.Fatalf("Registry len = %d, want %d", len(Registry), len(want))
	}
	for i, key := range want {
		if Registry[i].Key() != key {
			t.Errorf("Registry[%d].Key() = %q, want %q", i, Registry[i].Key(), key)
		}
	}
}

func TestValidate_WhenErrorIncludesHint(t *testing.T) {
	err := WhenType{}.Validate("invalid")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "AND/OR") {
		t.Errorf("error message missing AND/OR hint: %q", msg)
	}
	if !strings.Contains(msg, "comma separates") {
		t.Errorf("error message missing comma hint: %q", msg)
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
