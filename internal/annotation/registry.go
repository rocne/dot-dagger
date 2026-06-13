package annotation

import (
	"fmt"

	"github.com/rocne/dot-dagger/internal/predicate"
)

// Registry lists all annotation types the wizard presents, in menu order.
var Registry = []AnnotationType{
	WhenType{},
	AfterType{},
	RequireType{},
	RequestType{},
	ActionType{},
	NameType{},
	DisableType{},
}

// Compile-time interface checks.
var (
	_ AnnotationType = WhenType{}
	_ AnnotationType = AfterType{}
	_ AnnotationType = RequireType{}
	_ AnnotationType = RequestType{}
	_ AnnotationType = ActionType{}
	_ AnnotationType = NameType{}
	_ AnnotationType = DisableType{}
)

// --- WhenType ---

type WhenType struct{}

func (WhenType) Key()         string { return KeyWhen }
func (WhenType) Label()       string { return "When" }
func (WhenType) Description() string {
	return "Condition for when this file is active.\n\n" +
		predicate.IndentSyntaxHelp("  ") + "\n\n" +
		"Comma separates multiple values for ONE key. Use AND/OR to join two conditions."
}
func (WhenType) Kind()        InputKind { return KindText }
func (WhenType) Options()     []string  { return nil }
func (WhenType) Format(s string) string { return "# @when(" + s + ")" }

// Validate delegates to the canonical predicate parser so the wizard accepts
// exactly what the filter stage accepts (including call forms like
// exists(tmux)) and rejects what it rejects.
func (WhenType) Validate(s string) error {
	if _, err := predicate.Parse(s); err != nil {
		return fmt.Errorf(
			"@when: %w\nhint: key=value conditions joined with AND/OR; comma separates multiple values for one key (e.g. os=macos,linux)",
			err,
		)
	}
	return nil
}

// --- AfterType ---

type AfterType struct{}

func (AfterType) Key()         string { return KeyAfter }
func (AfterType) Label()       string { return "After" }
func (AfterType) Description() string { return "Logical name this file must load after" }
func (AfterType) Kind()        InputKind { return KindText }
func (AfterType) Options()     []string  { return nil }
func (AfterType) Format(s string) string { return "# @after(" + s + ")" }
func (AfterType) Validate(string) error  { return nil }

// --- RequireType ---

type RequireType struct{}

func (RequireType) Key()         string { return KeyRequire }
func (RequireType) Label()       string { return "Require" }
func (RequireType) Description() string { return "Package that must be installed for this file to be active" }
func (RequireType) Kind()        InputKind { return KindText }
func (RequireType) Options()     []string  { return nil }
func (RequireType) Format(s string) string { return "# @require(" + s + ")" }
func (RequireType) Validate(string) error  { return nil }

// --- RequestType ---

type RequestType struct{}

func (RequestType) Key()         string { return KeyRequest }
func (RequestType) Label()       string { return "Request" }
func (RequestType) Description() string { return "Package to install if missing (non-blocking)" }
func (RequestType) Kind()        InputKind { return KindText }
func (RequestType) Options()     []string  { return nil }
func (RequestType) Format(s string) string { return "# @request(" + s + ")" }
func (RequestType) Validate(string) error  { return nil }

// --- ActionType ---

type ActionType struct{}

func (ActionType) Key()         string { return KeyAction }
func (ActionType) Label()       string { return "Action" }
func (ActionType) Description() string { return "How dotd processes this file (source, no-source, or link)" }
func (ActionType) Kind()        InputKind { return KindChoice }
// NOTE: must stay in sync with the pipeline.Action* constants in
// internal/pipeline/walk.go; annotation cannot import pipeline (import cycle).
func (ActionType) Options()     []string  { return []string{"source", "no-source", "link"} }
func (ActionType) Format(s string) string { return "# @action(" + s + ")" }
func (ActionType) Validate(string) error  { return nil }

// --- NameType ---

type NameType struct{}

func (NameType) Key()         string { return KeyName }
func (NameType) Label()       string { return "Name" }
func (NameType) Description() string { return "Override logical name used in the dependency graph" }
func (NameType) Kind()        InputKind { return KindText }
func (NameType) Options()     []string  { return nil }
func (NameType) Format(s string) string { return "# @name(" + s + ")" }
func (NameType) Validate(string) error  { return nil }

// --- DisableType ---

type DisableType struct{}

func (DisableType) Key()         string { return KeyDisable }
func (DisableType) Label()       string { return "Disable" }
func (DisableType) Description() string { return "Exclude this file from all processing" }
func (DisableType) Kind()        InputKind { return KindBool }
func (DisableType) Options()     []string  { return nil }
func (DisableType) Format(string) string   { return "# @disable" }
func (DisableType) Validate(string) error  { return nil }
