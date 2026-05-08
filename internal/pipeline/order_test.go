package pipeline

import (
	"testing"
)

func makeOrderNode(name string) RawNode {
	return RawNode{Path: "/dots/" + name, LogicalName: name}
}

func makeAfterNode(name string, after ...string) RawNode {
	n := makeOrderNode(name)
	for _, a := range after {
		n.Actions = append(n.Actions, Action{Type: "_after_test_" + a})
	}
	// We embed after deps as a special field for the test.
	// Actually we need to use the After field on RawNode.
	// Let's just set it via a helper that sets After.
	_ = after
	return n
}

func namesOf(nodes []RawNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.LogicalName
	}
	return names
}

func TestOrder_AlphaNoAfter(t *testing.T) {
	nodes := []RawNode{
		makeOrderNode("c"),
		makeOrderNode("a"),
		makeOrderNode("b"),
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if !strSliceEq(namesOf(got), want) {
		t.Errorf("got %v, want %v", namesOf(got), want)
	}
}

func TestOrder_AfterConstraint(t *testing.T) {
	nodes := []RawNode{
		{Path: "/dots/b", LogicalName: "b", After: []string{"a"}},
		{Path: "/dots/a", LogicalName: "a"},
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if namesOf(got)[0] != "a" || namesOf(got)[1] != "b" {
		t.Errorf("got %v, want [a b]", namesOf(got))
	}
}

func TestOrder_Cycle_Error(t *testing.T) {
	nodes := []RawNode{
		{Path: "/dots/a", LogicalName: "a", After: []string{"b"}},
		{Path: "/dots/b", LogicalName: "b", After: []string{"a"}},
	}
	_, err := Order(nodes)
	if err == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestOrder_DuplicateName_Error(t *testing.T) {
	nodes := []RawNode{
		makeOrderNode("a"),
		makeOrderNode("a"),
	}
	_, err := Order(nodes)
	if err == nil {
		t.Error("expected duplicate name error, got nil")
	}
}

func TestOrder_PrefixAfter(t *testing.T) {
	// @after with trailing "/" matches all nodes under that path prefix.
	nodes := []RawNode{
		{Path: "/dots/b", LogicalName: "b", After: []string{"a/"}},
		{Path: "/dots/a", LogicalName: "a"},
		{Path: "/dots/a.extra", LogicalName: "a.extra"},
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatal(err)
	}
	names := namesOf(got)
	// b must come after both a and a.extra
	bIdx := indexOf(names, "b")
	aIdx := indexOf(names, "a")
	aExtraIdx := indexOf(names, "a.extra")
	if bIdx < aIdx || bIdx < aExtraIdx {
		t.Errorf("b should be last, got order %v", names)
	}
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
