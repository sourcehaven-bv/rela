package mapimport

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// redactGate reads as inner does and withholds the values of fields, as a
// `visible:` grant does.
type redactGate struct {
	inner  ReadGate
	fields map[string]bool
}

func (g redactGate) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	e, err := g.inner.GetAddress(ctx, addr)
	if err != nil {
		return nil, err
	}
	out := *e
	out.Properties = maps.Clone(e.Properties)
	out.Redacted = nil
	for name := range g.fields {
		delete(out.Properties, name)
		out.Redacted = append(out.Redacted, name)
	}
	slices.Sort(out.Redacted)
	return &out, nil
}

// TestHiddenProperty_NotCompared pins that the plan neither shows nor
// compares a property value the principal may not see.
func TestHiddenProperty_NotCompared(t *testing.T) {
	w := newWorld(t)
	w.create("control", "", map[string]any{"oscal_id": "c1", "title": "Old", "weight": int64(9)})
	w.gate = redactGate{inner: w.gate, fields: map[string]bool{"weight": true}}

	weighted := w.mapping(controlsMapping + "      weight: weight\n")
	p := w.plan(weighted, FormatJSON, `{"controls": [{"id": "c1", "title": "Old", "weight": 3}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "property weight") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "9") || len(p.Updates)+p.Unchanged != 0 {
		t.Fatalf("plan shows or compares the hidden value: %s", data)
	}

	// A mapping that leaves the hidden property alone updates as usual.
	p = w.plan(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1", "title": "New"}]}`)
	if len(p.Errors) != 0 || len(p.Updates) != 1 || p.Updates[0].Properties[0].Old != "Old" {
		t.Fatalf("plan = %+v", p)
	}
}

// TestHiddenScope_NotNamed pins that a row hiding a scope property is
// treated as unreadable, so the plan does not reveal its scope value.
func TestHiddenScope_NotNamed(t *testing.T) {
	w := newWorld(t)
	w.create("control", "", map[string]any{"oscal_id": "c9", "catalog": "x"})
	w.gate = redactGate{inner: w.gate, fields: map[string]bool{"catalog": true}}
	m := w.mapping(controlsMapping + "    scope: {catalog: x}\n")
	if p := w.plan(m, FormatJSON, `{"controls": []}`); len(p.Missing) != 0 {
		t.Fatalf("missing = %+v", p.Missing)
	}
	p := w.plan(m, FormatJSON, `{"controls": [{"id": "c9"}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "cannot read") {
		t.Fatalf("errors = %+v", p.Errors)
	}
}

// TestHiddenProperty_RealPolicy runs the hidden-property case through the
// real ACL path: an acl.yaml `visible:` grant, the policy redactor and the
// declarative gate.
func TestHiddenProperty_RealPolicy(t *testing.T) {
	const policy = `roles:
  reader:
    read: ["*"]
    update: [control]
    visible:
      control:
        - field: oscal_id
        - field: title
assignments:
  alice: reader
`
	w := aclWorld(t, policy, "alice")
	if err := w.st.CreateEntity(w.ctx, &entity.Entity{
		ID: "CTL-1", Type: "control",
		Properties: map[string]any{"oscal_id": "c1", "title": "Old", "weight": int64(987)},
	}); err != nil {
		t.Fatal(err)
	}

	weighted := w.mapping(controlsMapping + "      weight: weight\n")
	p := w.plan(weighted, FormatJSON, `{"controls": [{"id": "c1", "title": "Old", "weight": 3}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "property weight") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "987") || len(p.Updates)+p.Unchanged != 0 {
		t.Fatalf("plan shows or compares the hidden value: %s", data)
	}

	// A mapping of visible fields only updates, and the hidden value stays.
	w.run(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1", "title": "New"}]}`)
	got := w.byKey("control", "c1")
	if got == nil || got.Properties["title"] != "New" || normalize(got.Properties["weight"]) != "987" {
		t.Fatalf("stored = %+v", got)
	}
}

// TestRelationErrors_DropRecordsInChain pins that a record whose relation
// fails is left out, and that a record relating to it fails in turn.
func TestRelationErrors_DropRecordsInChain(t *testing.T) {
	w := newWorld(t)
	w.create("risk", "RSK-1", map[string]any{"ref": map[string]any{"id": "R-1"}})
	src := `
entities:
  controls:
    type: control
    select: $.controls
    key: oscal_id
    properties:
      oscal_id: id
    relations:
      - type: mitigates
        target_type: risk
        target_key: ref
        key: risk
      - type: refines
        target: controls
        key: needs
`
	// c2 names an unknown risk. c1 refines c2, so it fails once c2 is out.
	p := w.plan(w.mapping(src), FormatJSON, `{"controls": [
    {"id": "c1", "needs": "c2", "risk": "R-1"},
    {"id": "c2", "needs": "c1", "risk": "R-9"}
  ]}`)
	if len(p.Errors) != 2 || !strings.Contains(p.Errors[1].Message, "has record errors") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	if p.HasChanges() || len(p.Creates) != 0 || len(p.RelationAdds) != 0 {
		t.Fatalf("plan = %+v", p)
	}
}

// TestScopeAndHookValuesAreConverted pins that a scope constant and a hook's
// text reach an integer property as integers, so a re-run is unchanged.
func TestScopeAndHookValuesAreConverted(t *testing.T) {
	w := newWorld(t)
	w.files["scripts/h.lua"] = `
function transform(d, rec)
  d.properties.note = "n"
  return d
end`
	src := `
entities:
  controls:
    type: control
    select: $.controls
    key: oscal_id
    scope: {weight: 3}
    properties:
      oscal_id: id
    hook: scripts/h.lua
`
	m := w.mapping(src)
	w.run(m, FormatJSON, `{"controls": [{"id": "c1"}]}`)
	if got := w.byKey("control", "c1").Properties["weight"]; normalize(got) != "3" {
		t.Fatalf("weight = %#v", got)
	}
	if p := w.plan(m, FormatJSON, `{"controls": [{"id": "c1"}]}`); p.Unchanged != 1 || p.HasChanges() {
		t.Fatalf("re-run plan = %+v", p)
	}

	v, err := hookValue(w.meta.Entities["control"].Properties["weight"], "5")
	if err != nil || v != int64(5) {
		t.Fatalf("hookValue = %#v, %v", v, err)
	}
	if _, err := hookValue(w.meta.Entities["control"].Properties["weight"], "five"); err == nil {
		t.Fatal("hookValue accepted text that is not an integer")
	}
}

func TestCoerceInteger(t *testing.T) {
	pd := testMeta(t).Entities["control"].Properties["weight"]
	for in, want := range map[string]int64{"7": 7, "3.0": 3, "1e3": 1000} {
		if got, err := coerce(pd, in); err != nil || got != want {
			t.Errorf("coerce(%q) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"3.5", "x", "1e30"} {
		if _, err := coerce(pd, in); err == nil {
			t.Errorf("coerce(%q) accepted it", in)
		}
	}
}

func TestDecodeYAML_CountsBeforeParsing(t *testing.T) {
	_, err := Decode([]byte(strings.Repeat("- 1\n", 50)), FormatYAML, Limits{MaxNodes: 10})
	if _, ok := errors.AsType[*LimitError](err); !ok {
		t.Fatalf("err = %v, want a LimitError", err)
	}
}

func TestBuild_RefusesUnparsedMapping(t *testing.T) {
	w := newWorld(t)
	m := &Mapping{Name: "x", Entities: map[string]*EntityMapping{"c": {Type: "control", Select: "$"}}}
	root, err := Decode([]byte(`{}`), FormatJSON, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(w.ctx, Deps{Meta: w.meta, Store: w.st, Gate: w.gate}, m, root); err == nil {
		t.Fatal("Build accepted a mapping that did not come from Parse")
	}
}
