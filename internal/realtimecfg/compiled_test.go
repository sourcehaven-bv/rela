package realtimecfg

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// The standard library has no function taking a record, so in realtime.yaml
// every whole use of `entity` is a traversal. The allowlist must still
// refuse a whole use on its own: a host function added later that takes the
// record would read fields the rule's Fields list does not name.
func TestAllowRule_WholeUseWithoutTraversal(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"status": predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	if err := env.DeclareFunc("inspect", predicate.FuncSig{
		Params: []predicate.Type{predicate.RecordType{}}, Return: predicate.BoolType,
	}); err != nil {
		t.Fatal(err)
	}
	prog, err := predicate.Compile(env, "inspect(entity)")
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Traversals()) != 0 {
		t.Fatal("the program has a traversal; the test needs a whole use without one")
	}
	v := &validator{meta: testMeta(t)}
	if v.allowRule("rule", "datalek", prog) {
		t.Fatal("allowRule accepted a whole use of entity")
	}
	var whole bool
	for _, p := range v.problems {
		if strings.Contains(p.Reason, "follow relations") {
			t.Errorf("a program without a traversal was refused as one: %s", p)
		}
		whole = whole || strings.Contains(p.Reason, "only as entity.<property>")
	}
	if !whole {
		t.Errorf("no whole-use problem; got %v", v.problems)
	}
}

func TestSource_NeedsCeilingCheck(t *testing.T) {
	kinds := []SourceKind{SourcePermission, SourceAttached, SourceEntityProperty, SourcePrincipalType, SourceCreator}
	for _, k := range kinds {
		s := Source{Kind: k}
		if !s.NeedsCeilingCheck(CapabilityUpdate) {
			t.Errorf("kind %d: a write must always check the ceiling's update axis", k)
		}
		if want := k != SourceAttached; s.NeedsCeilingCheck(CapabilityRead) != want {
			t.Errorf("kind %d: NeedsCeilingCheck(read) = %v; want %v", k, !want, want)
		}
	}
}

// Every accessor returns a copy: writing through a returned value must not
// change what the next caller sees.
func TestCompiled_AccessorsReturnCopies(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	room, _ := c.ChannelType("incident-room")
	room.AttachTo[0] = "x"
	room.Roles["lead"] = Role{Name: "hijacked"}
	room.Read.RequireAnchorRead = false
	su := room.MessageTypes["status-update"]
	su.Post[0] = "x"
	su.Properties["severity"].Values[0] = "x"
	su.PromoteTo.Map["title"] = "x"
	su.Actions["accept"].Permission = "x"
	su.Actions["accept"].Roles = append(su.Actions["accept"].Roles, "x")
	room.Lifecycle.Close.Roles[0] = "x"
	room.Lifecycle.Close.Auto.When["datalek"].Fields[0] = "x"
	room.MessageOps.EditOwn.Roles[0] = "x"
	room.MessageOps.DeleteOwn.Roles[0] = "x"
	room.MessageOps.Remove.Roles[0] = "x"
	delete(room.MessageTypes, "text")
	c.ChannelTypes()[1].Create.Roles[0] = "x"
	tr := c.Triggers()[0]
	tr.Props["summary"] = "x"
	tr.When.Fields[0] = "x"
	c.UsedPermissions()[0] = "x"

	again, _ := c.ChannelType("incident-room")
	asu := again.MessageTypes["status-update"]
	checks := map[string]bool{
		"attach_to":       again.AttachTo[0] == "datalek",
		"roles":           again.Roles["lead"].Name == "lead",
		"read":            again.Read.RequireAnchorRead,
		"post":            asu.Post[0] == "lead",
		"enum values":     asu.Properties["severity"].Values[0] == "low",
		"promote map":     asu.PromoteTo.Map["title"] == "summary",
		"action":          asu.Actions["accept"].Permission == "incident.decide" && len(asu.Actions["accept"].Roles) == 0,
		"lifecycle roles": again.Lifecycle.Close.Roles[0] == "lead",
		"auto fields":     again.Lifecycle.Close.Auto.When["datalek"].Fields[0] == "status",
		"edit_own":        again.MessageOps.EditOwn.Roles[0] == "lead",
		"delete_own":      again.MessageOps.DeleteOwn.Roles[0] == "lead",
		"remove":          again.MessageOps.Remove.Roles[0] == "lead",
		"message types":   again.MessageTypes["text"] != nil,
		"create":          c.ChannelTypes()[1].Create.Roles[0] == "editor",
		"trigger props":   c.Triggers()[0].Props["summary"] == "Severity raised to high",
		"trigger fields":  c.Triggers()[0].When.Fields[0] == "due",
		"permissions":     c.UsedPermissions()[0] == "incident.decide",
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s: a write through a returned value changed the compiled config", name)
		}
	}
	if _, ok := c.ChannelType("nope"); ok {
		t.Error("unknown channel type found")
	}
}

func TestShapeMismatches(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]TypeShape{
		// configured family/one: an anchor mismatch
		"incident-room": {Anchor: AnchorFace, Cardinality: CardinalityOne},
		// configured face/many: a cardinality mismatch
		"page-threads": {Anchor: AnchorFace, Cardinality: CardinalityOne},
		// no longer declared: not a mismatch
		"gone": {Anchor: AnchorFamily, Cardinality: CardinalityMany},
	}
	got := ShapeMismatches(c, stored)
	want := []ShapeMismatch{
		{
			ChannelType: "incident-room",
			Stored:      TypeShape{Anchor: AnchorFace, Cardinality: CardinalityOne},
			Configured:  TypeShape{Anchor: AnchorFamily, Cardinality: CardinalityOne},
		},
		{
			ChannelType: "page-threads",
			Stored:      TypeShape{Anchor: AnchorFace, Cardinality: CardinalityOne},
			Configured:  TypeShape{Anchor: AnchorFace, Cardinality: CardinalityMany},
		},
	}
	type key struct {
		name               string
		stored, configured [2]string
	}
	project := func(ms []ShapeMismatch) []key {
		var out []key
		for _, m := range ms {
			out = append(out, key{m.ChannelType,
				[2]string{string(m.Stored.Anchor), string(m.Stored.Cardinality)},
				[2]string{string(m.Configured.Anchor), string(m.Configured.Cardinality)}})
		}
		return out
	}
	if !slices.Equal(project(got), project(want)) {
		t.Fatalf("ShapeMismatches = %+v; want %+v", got, want)
	}
	err = ShapeMismatchError(got)
	assertProblem(t, err, "channel_types.incident-room.anchor", "migrating them to another anchor is not supported yet")
	assertProblem(t, err, "channel_types.page-threads.cardinality", "cannot change from one to many")
	if strings.Contains(err.Error(), "restart") {
		t.Errorf("the refusal suggests a restart: %v", err)
	}
	if ShapeMismatchError(nil) != nil || ShapeMismatches(nil, stored) != nil || ShapeMismatches(c, nil) != nil {
		t.Error("no mismatches expected for empty input")
	}
}

// A stored shape with an empty cardinality was recorded before the channel
// store kept one. It matches any configured cardinality, but the anchor is
// still compared.
func TestShapeMismatchesUnknownStoredCardinality(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	// incident-room is configured family/one, page-threads face/many.
	if got := ShapeMismatches(c, map[string]TypeShape{
		"incident-room": {Anchor: AnchorFamily},
		"page-threads":  {Anchor: AnchorFace},
	}); len(got) != 0 {
		t.Fatalf("ShapeMismatches = %+v; want none", got)
	}
	got := ShapeMismatches(c, map[string]TypeShape{"incident-room": {Anchor: AnchorFace}})
	if len(got) != 1 {
		t.Fatalf("ShapeMismatches = %+v; want one anchor mismatch", got)
	}
	problems := got[0].Problems()
	if len(problems) != 1 || problems[0].Key != "channel_types.incident-room.anchor" {
		t.Fatalf("Problems = %+v; want the anchor only", problems)
	}
}

// billionLaughs builds a file whose channel types alias a message type map
// that aliases a property map, n entries at every level. Following the
// aliases visits about n^3 nodes from a file of 3n lines.
func billionLaughs(n int) string {
	var b strings.Builder
	b.WriteString("version: 1\nx-props: &props\n")
	for i := range n {
		fmt.Fprintf(&b, "  p%d: {type: string}\n", i)
	}
	b.WriteString("x-types: &types\n")
	for i := range n {
		fmt.Fprintf(&b, "  m%d: {post: [r], properties: *props}\n", i)
	}
	b.WriteString("channel_types:\n")
	for i := range n {
		fmt.Fprintf(&b, "  c%d: {attach_to: [datalek], message_types: *types}\n", i)
	}
	return b.String()
}

func TestParse_BillionLaughsIsBounded(t *testing.T) {
	_, err := Parse([]byte(billionLaughs(200)), testMeta(t), nil)
	assertProblem(t, err, "", "expands to more than")
}

func TestScanPermissions(t *testing.T) {
	got, err := ScanPermissions([]byte(`version: 1
channel_types:
  room:
    attach_to: [nope]
    roles: { a: { permission: b.perm }, c: { permission: a.perm } }
    lifecycle: { delete: &d { permission: a.perm } }
    colr: { permission: misspelled.parent }
    extra: *d
`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.perm", "b.perm", "misspelled.parent"}; !slices.Equal(got, want) {
		t.Errorf("ScanPermissions = %q; want %q", got, want)
	}
	if _, err := ScanPermissions([]byte("version: [1")); err == nil {
		t.Error("unparseable YAML was scanned")
	}
	if _, err := ScanPermissions([]byte(billionLaughs(200))); err == nil {
		t.Error("an exponentially aliased file was scanned to the end")
	}
}
