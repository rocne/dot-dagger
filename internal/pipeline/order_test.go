package pipeline

import (
	"fmt"
	"math/rand"
	"testing"
)

func makeOrderNode(name string) RawNode {
	return RawNode{Path: "/dots/" + name, LogicalName: name}
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

// TestOrder_UnknownAfterRef verifies that an @after reference to a nonexistent
// node name is silently ignored — no edge is created and no error is returned
// (AUDIT-051). The ordering still respects any valid constraints.
func TestOrder_UnknownAfterRef(t *testing.T) {
	nodes := []RawNode{
		{Path: "/dots/a", LogicalName: "a", After: []string{"nonexistent"}},
		{Path: "/dots/b", LogicalName: "b"},
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatalf("Order with unknown @after ref: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got))
	}
}

// TestOrder_SelfRefIgnored verifies that a node whose @after ref resolves to
// itself (by logical name) does not create a self-edge and does not cause a
// cycle error (AUDIT-051).
func TestOrder_SelfRefIgnored(t *testing.T) {
	nodes := []RawNode{
		{Path: "/dots/a", LogicalName: "a", After: []string{"a"}}, // literal self-ref
		{Path: "/dots/b", LogicalName: "b"},
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatalf("Order with self @after ref: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got))
	}
}

// TestOrder_PrefixSelfRefIgnored verifies that a node whose prefix @after ref
// would match itself (e.g. node "a.x" with After: ["a/"]) does not create a
// self-loop. The prefix match is a dot-prefix comparison (AUDIT-051).
func TestOrder_PrefixSelfRefIgnored(t *testing.T) {
	// Node "a.sub" references prefix "a/" which expands to all logical names
	// starting with "a" — including itself. Self-match must be dropped.
	nodes := []RawNode{
		{Path: "/dots/a.sub", LogicalName: "a.sub", After: []string{"a/"}},
		{Path: "/dots/b", LogicalName: "b"},
	}
	got, err := Order(nodes)
	if err != nil {
		t.Fatalf("Order with prefix self-ref: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(got))
	}
}

// TestResolveAfterRef_PrefixBoundary verifies that a prefix ref "foo/" matches
// the dir node "foo" and its descendants "foo.*", but NOT an unrelated sibling
// that merely shares the name prefix ("foobar"). The match must be on a dot
// boundary, not a bare string prefix.
func TestResolveAfterRef_PrefixBoundary(t *testing.T) {
	nodes := []RawNode{
		{LogicalName: "foo"},
		{LogicalName: "foo.bar"},
		{LogicalName: "foobar"},
	}
	got := ResolveAfterRef("foo/", nodes)
	if indexOf(got, "foobar") != -1 {
		t.Errorf("prefix \"foo/\" wrongly matched sibling \"foobar\": %v", got)
	}
	if indexOf(got, "foo.bar") == -1 {
		t.Errorf("prefix \"foo/\" should match descendant \"foo.bar\": %v", got)
	}
	if indexOf(got, "foo") == -1 {
		t.Errorf("prefix \"foo/\" should match the dir node \"foo\": %v", got)
	}
}

// TestResolveAfterRef_EmptyMatchSet verifies that ResolveAfterRef returns an
// empty slice when the prefix doesn't match any node — no error, no panic
// (AUDIT-051).
func TestResolveAfterRef_EmptyMatchSet(t *testing.T) {
	nodes := []RawNode{
		{Path: "/dots/x", LogicalName: "x"},
		{Path: "/dots/y", LogicalName: "y"},
	}
	matches := ResolveAfterRef("z/", nodes) // prefix "z" matches nothing
	if len(matches) != 0 {
		t.Errorf("ResolveAfterRef empty match: got %v, want empty slice", matches)
	}
}

// TestOrder_DeterministicTieBreak verifies that when 3+ nodes become ready
// simultaneously after their shared dependency resolves, Order() produces
// identical alphabetical output across repeated runs regardless of input order.
// This covers the tie-break logic at order.go:60, 76-77, 112 (AUDIT-050).
func TestOrder_DeterministicTieBreak(t *testing.T) {
	// One root with no dependencies; four siblings that all @after root.
	// Sibling names chosen so alphabetical order is unambiguous: a < b < c < d.
	base := []RawNode{
		{Path: "/dots/dep", LogicalName: "dep"},
		{Path: "/dots/a", LogicalName: "a", After: []string{"dep"}},
		{Path: "/dots/b", LogicalName: "b", After: []string{"dep"}},
		{Path: "/dots/c", LogicalName: "c", After: []string{"dep"}},
		{Path: "/dots/d", LogicalName: "d", After: []string{"dep"}},
	}

	want := []string{"dep", "a", "b", "c", "d"}

	rng := rand.New(rand.NewSource(42))

	const iterations = 100
	for i := 0; i < iterations; i++ {
		// Shuffle a copy of the input to stress against accidental sort stability.
		input := make([]RawNode, len(base))
		copy(input, base)
		rng.Shuffle(len(input), func(x, y int) { input[x], input[y] = input[y], input[x] })

		got, err := Order(input)
		if err != nil {
			t.Fatalf("iteration %d: Order() error: %v", i, err)
		}
		names := namesOf(got)
		if !strSliceEq(names, want) {
			t.Fatalf("iteration %d: got %v, want %v (input was shuffled)", i, names, want)
		}
	}
}

// TestOrder_Property generates random acyclic DAGs — every @after edge points to
// a strictly-earlier-indexed node, so a cycle is impossible — and checks the
// invariants Order must hold for every shape, not just the fixed examples above:
//
//	A. determinism: reordering the input never changes the output;
//	B. dependency: every resolved @after dependency precedes its dependent;
//	C. completeness: acyclic input yields every node exactly once.
func TestOrder_Property(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const graphs = 300
	for g := 0; g < graphs; g++ {
		n := 1 + rng.Intn(12) // 1..12 nodes
		names := make([]string, n)
		for i := range names {
			names[i] = fmt.Sprintf("n%02d", i)
		}
		nodes := make([]RawNode, n)
		for i := 0; i < n; i++ {
			var after []string
			for j := 0; j < i; j++ { // depend only on earlier nodes → acyclic
				if rng.Intn(3) == 0 {
					after = append(after, names[j])
				}
			}
			nodes[i] = RawNode{Path: "/dots/" + names[i], LogicalName: names[i], After: after}
		}

		canonical, err := Order(nodes)
		if err != nil {
			t.Fatalf("graph %d: Order error on acyclic input: %v", g, err)
		}

		// C: completeness.
		if len(canonical) != n {
			t.Fatalf("graph %d: result has %d nodes, want %d", g, len(canonical), n)
		}

		// B: every @after dependency precedes its dependent.
		pos := make(map[string]int, n)
		for i, nd := range canonical {
			pos[nd.LogicalName] = i
		}
		for _, nd := range nodes {
			for _, dep := range nd.After {
				if pos[dep] >= pos[nd.LogicalName] {
					t.Fatalf("graph %d: %q must precede %q but does not: %v",
						g, dep, nd.LogicalName, namesOf(canonical))
				}
			}
		}

		// A: determinism — shuffled inputs produce the identical order.
		want := namesOf(canonical)
		for s := 0; s < 5; s++ {
			shuffled := make([]RawNode, n)
			copy(shuffled, nodes)
			rng.Shuffle(n, func(x, y int) { shuffled[x], shuffled[y] = shuffled[y], shuffled[x] })
			got, err := Order(shuffled)
			if err != nil {
				t.Fatalf("graph %d shuffle %d: Order error: %v", g, s, err)
			}
			if !strSliceEq(namesOf(got), want) {
				t.Fatalf("graph %d shuffle %d: non-deterministic order\n got=%v\nwant=%v",
					g, s, namesOf(got), want)
			}
		}
	}
}
