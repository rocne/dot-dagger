package pipeline

import "testing"

func makeNode(name, when string, actions ...Action) RawNode {
	return RawNode{
		Path:          "/dots/" + name,
		LogicalName:   name,
		EffectiveWhen: when,
		Actions:       actions,
	}
}

func TestFilter_EmptyWhen_AlwaysActive(t *testing.T) {
	nodes := []RawNode{makeNode("base", "")}
	got, err := Filter(nodes, map[string]string{"os": "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 node, got %d", len(got))
	}
}

func TestFilter_WhenMatch_Active(t *testing.T) {
	nodes := []RawNode{makeNode("macos", "(os=macos)")}
	got, err := Filter(nodes, map[string]string{"os": "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LogicalName != "macos" {
		t.Errorf("expected macos node active, got %v", got)
	}
}

func TestFilter_WhenMismatch_Inactive(t *testing.T) {
	nodes := []RawNode{makeNode("macos", "(os=macos)")}
	got, err := Filter(nodes, map[string]string{"os": "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(got))
	}
}

func TestFilter_AndExpression(t *testing.T) {
	nodes := []RawNode{makeNode("work-macos", "(context=work) AND (os=macos)")}

	// Both match → active.
	got, err := Filter(nodes, map[string]string{"context": "work", "os": "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 node when both match, got %d", len(got))
	}

	// Only one matches → inactive.
	got, err = Filter(nodes, map[string]string{"context": "personal", "os": "macos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 nodes when one doesn't match, got %d", len(got))
	}
}

func TestFilter_MixedNodes(t *testing.T) {
	nodes := []RawNode{
		makeNode("base", ""),
		makeNode("macos", "(os=macos)"),
		makeNode("linux", "(os=linux)"),
	}
	got, err := Filter(nodes, map[string]string{"os": "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 nodes (base + linux), got %d", len(got))
	}
}

func TestCollectMissingKeys_SingleMissing(t *testing.T) {
	nodes := []RawNode{makeNode("a", "context=work")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "context" {
		t.Errorf("expected [context], got %v", got)
	}
}

func TestCollectMissingKeys_AndBothMissing(t *testing.T) {
	// Both sides of AND must be found — no short-circuit.
	nodes := []RawNode{makeNode("a", "context=work AND machine=laptop")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 missing keys, got %v", got)
	}
	keys := map[string]bool{got[0]: true, got[1]: true}
	if !keys["context"] || !keys["machine"] {
		t.Errorf("expected context and machine, got %v", got)
	}
}

func TestCollectMissingKeys_Dedup(t *testing.T) {
	// Same key referenced in two nodes — returned once.
	nodes := []RawNode{
		makeNode("a", "context=work"),
		makeNode("b", "context=personal"),
	}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "context" {
		t.Errorf("expected [context] once, got %v", got)
	}
}

func TestCollectMissingKeys_NoneMissing(t *testing.T) {
	nodes := []RawNode{makeNode("a", "context=work")}
	got, err := CollectMissingKeys(nodes, map[string]string{"context": "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCollectMissingKeys_EmptyWhenSkipped(t *testing.T) {
	nodes := []RawNode{makeNode("base", "")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty for empty-when node, got %v", got)
	}
}

func TestCollectMissingKeys_ParseError(t *testing.T) {
	nodes := []RawNode{makeNode("bad", "!!invalid!!")}
	_, err := CollectMissingKeys(nodes, map[string]string{})
	if err == nil {
		t.Error("expected parse error, got nil")
	}
}

func TestCollectMissingKeys_PartiallySet(t *testing.T) {
	// context set, machine missing.
	nodes := []RawNode{makeNode("a", "context=work AND machine=laptop")}
	got, err := CollectMissingKeys(nodes, map[string]string{"context": "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "machine" {
		t.Errorf("expected [machine], got %v", got)
	}
}

// TestCollectMissingKeys_ORCaveat documents the OR-collection caveat (AUDIT-053):
// keys from ALL OR branches are collected even when one branch is already
// satisfied by env. This is AST-structural collection, not short-circuit
// evaluation. The caller may prompt for "unnecessary" keys in rare OR-across-
// different-keys configurations — this is the intentional documented trade-off.
func TestCollectMissingKeys_ORCaveat(t *testing.T) {
	// os=linux satisfies the OR predicate at eval time, yet distro must still
	// appear in the missing-keys output because CollectMissingKeys uses the AST
	// (Keys() on OrExpr collects from all branches).
	nodes := []RawNode{makeNode("a", "os=linux OR distro=fedora")}
	env := map[string]string{"os": "linux"}

	got, err := CollectMissingKeys(nodes, env)
	if err != nil {
		t.Fatalf("CollectMissingKeys error: %v", err)
	}
	if len(got) != 1 || got[0] != "distro" {
		t.Errorf("OR caveat: got %v, want [distro] (distro is absent even though OR is satisfied by os=linux)", got)
	}
}

// TestCollectMissingKeys_ORAllBranchesMissing verifies all OR-branch keys are
// reported when both are absent.
func TestCollectMissingKeys_ORAllBranchesMissing(t *testing.T) {
	nodes := []RawNode{makeNode("a", "os=linux OR distro=fedora")}
	got, err := CollectMissingKeys(nodes, map[string]string{})
	if err != nil {
		t.Fatalf("CollectMissingKeys error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("OR all missing: got %v (len=%d), want 2 keys", got, len(got))
	}
	keyset := map[string]bool{}
	for _, k := range got {
		keyset[k] = true
	}
	if !keyset["os"] || !keyset["distro"] {
		t.Errorf("OR all missing: got %v, expected both os and distro", got)
	}
}
