package mapimport

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestOSCALCatalog imports the example catalog in its JSON and its XML form,
// checks the result, and checks that a second run plans nothing.
func TestOSCALCatalog(t *testing.T) {
	cases := []struct {
		name, mapping, input string
		format               Format
	}{
		{"json", "oscal/mapping.yaml", "oscal/catalog.json", FormatJSON},
		{"xml", "oscal/mapping-xml.yaml", "oscal/catalog.xml", FormatXML},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			m := w.mapping(readTestdata(t, tc.mapping))
			data := readTestdata(t, tc.input)
			p := w.run(m, tc.format, data)
			if len(p.Creates) != 6 || p.Skipped != 1 {
				t.Fatalf("creates=%d skipped=%d, want 6 and 1 (org-3 is withdrawn)", len(p.Creates), p.Skipped)
			}
			if n := w.controls(); n != 4 {
				t.Fatalf("controls = %d, want 4", n)
			}
			ctl := w.byKey("control", "org-1")
			if ctl == nil {
				t.Fatal("org-1 not imported")
			}
			if ctl.Properties["title"] != "Policies for information security" || ctl.Properties["label"] != "5.1" ||
				ctl.Properties["catalog"] != "example" {

				t.Fatalf("org-1 properties = %v", ctl.Properties)
			}
			if !strings.HasPrefix(ctl.Content, "Define, approve and publish") {
				t.Fatalf("org-1 content = %q", ctl.Content)
			}
			if tech := w.byKey("control", "tech-1"); tech.Properties["label"] != "tech-1" {
				t.Fatalf("hook default label = %v, want tech-1", tech.Properties["label"])
			}
			grp := w.byKey("control-group", "org")
			enh := w.byKey("control", "org-1.1")
			partOf := w.relations("part-of")
			if len(partOf) != 3 || !slices.Contains(partOf, ctl.ID+"->"+grp.ID) {
				t.Fatalf("part-of = %v", partOf)
			}
			if refines := w.relations("refines"); !slices.Equal(refines, []string{enh.ID + "->" + ctl.ID}) {
				t.Fatalf("refines = %v", refines)
			}

			again := w.plan(m, tc.format, data)
			if again.HasChanges() || again.Unchanged != 6 || len(again.Errors) > 0 || len(again.Stale) > 0 {
				t.Fatalf("second run plans changes: %+v", again)
			}
		})
	}
}

const controlsMapping = `
entities:
  controls:
    type: control
    select: $.controls
    key: oscal_id
    properties:
      oscal_id: id
      title: title
`

// TestReimport_UpdatesOnlyChangedProperties pins that an update names the
// changed property with its old and new value, and that a property the
// mapping does not own survives.
func TestReimport_UpdatesOnlyChangedProperties(t *testing.T) {
	w := newWorld(t)
	m := w.mapping(controlsMapping)
	w.run(m, FormatJSON, `{"controls": [{"id": "c1", "title": "Old"}, {"id": "c2", "title": "Same"}]}`)
	c1 := w.byKey("control", "c1")
	if _, err := w.mgr.PatchEntity(w.ctx, c1.ID, entity.Patch{Properties: map[string]any{"note": "mine"}}); err != nil {
		t.Fatal(err)
	}

	p := w.plan(m, FormatJSON, `{"controls": [{"id": "c1", "title": "New"}, {"id": "c2", "title": "Same"}]}`)
	if len(p.Updates) != 1 || p.Unchanged != 1 || len(p.Creates) != 0 {
		t.Fatalf("plan = %+v", p)
	}
	want := []PropertyChange{{Name: "title", Old: "Old", New: "New"}}
	if !slices.Equal(p.Updates[0].Properties, want) {
		t.Fatalf("changes = %+v, want %+v", p.Updates[0].Properties, want)
	}
	w.apply(p)
	got := w.byKey("control", "c1")
	if got.Properties["title"] != "New" || got.Properties["note"] != "mine" {
		t.Fatalf("after update: %v", got.Properties)
	}
}

func TestDryRun_WritesNothing(t *testing.T) {
	w := newWorld(t)
	p := w.plan(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1", "title": "T"}]}`)
	if len(p.Creates) != 1 {
		t.Fatalf("plan = %+v", p)
	}
	if n := w.controls(); n != 0 {
		t.Fatalf("Build wrote %d entities", n)
	}
}

// TestAbsentValues pins that a value the source lacks leaves the stored one
// alone, unless the mapping sets clear_missing.
func TestAbsentValues(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "kept", true: "cleared"}[clear], func(t *testing.T) {
			w := newWorld(t)
			src := controlsMapping
			if clear {
				src = strings.Replace(src, "title: title", "title: {path: title, clear_missing: true}", 1)
			}
			m := w.mapping(src)
			w.run(m, FormatJSON, `{"controls": [{"id": "c1", "title": "T"}]}`)
			p := w.run(m, FormatJSON, `{"controls": [{"id": "c1"}]}`)
			got := w.byKey("control", "c1").Properties["title"]
			if clear {
				if len(p.Updates) != 1 || !p.Updates[0].Properties[0].Cleared || got != nil {
					t.Fatalf("title = %v, plan %+v", got, p.Updates)
				}
				return
			}
			if p.HasChanges() || got != "T" {
				t.Fatalf("title = %v, plan %+v", got, p.Updates)
			}
		})
	}
}

func TestTypedValues(t *testing.T) {
	w := newWorld(t)
	m := w.mapping(`
entities:
  controls:
    type: control
    select: $.controls
    key: oscal_id
    properties:
      oscal_id: id
      weight: weight
      tags: tags
      title: {template: "{{id}}: {{name}}"}
      note: {path: lines, join: " / "}
`)
	data := `{"controls": [{"id": "c1", "name": "One", "weight": 3, "tags": ["a", "b"], "lines": ["x", "y"]}]}`
	w.run(m, FormatJSON, data)
	got := w.byKey("control", "c1").Properties
	if got["weight"] != int64(3) && got["weight"] != 3 {
		t.Fatalf("weight = %#v", got["weight"])
	}
	if normalize(got["tags"]) != "a\x1fb" || got["title"] != "c1: One" || got["note"] != "x / y" {
		t.Fatalf("properties = %#v", got)
	}
	if p := w.plan(m, FormatJSON, data); p.HasChanges() {
		t.Fatalf("typed values plan changes on a second run: %+v", p.Updates)
	}
	bad := w.plan(m, FormatJSON, `{"controls": [{"id": "c2", "weight": "heavy"}]}`)
	if len(bad.Errors) != 1 || !strings.Contains(bad.Errors[0].Message, "not an integer") {
		t.Fatalf("errors = %+v", bad.Errors)
	}
}

// TestRelationToExistingType links controls to risks the import does not
// own, by the risks' external ref.
func TestRelationToExistingType(t *testing.T) {
	w := newWorld(t)
	w.create("risk", "RSK-1", map[string]any{"ref": map[string]any{"id": "R-1"}, "title": "Leak"})
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
        key: risks
`
	m := w.mapping(src)
	p := w.plan(m, FormatJSON, `{"controls": [{"id": "c1", "risks": ["R-1"]}, {"id": "c2", "risks": ["R-9"]}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, `no risk with ref "R-9"`) {
		t.Fatalf("errors = %+v", p.Errors)
	}
	if _, err := Apply(w.ctx, w.mgr, p, ApplyOptions{}); !errors.Is(err, ErrPlanHasErrors) {
		t.Fatalf("Apply on a plan with errors = %v", err)
	}
	res, err := Apply(w.ctx, w.mgr, p, ApplyOptions{AllowErrors: true})
	// The record whose relation failed is left out whole: c2 is not created.
	if err != nil || res.RelationsAdded != 1 || res.Created != 1 {
		t.Fatalf("Apply = %+v, %v", res, err)
	}
	if w.byKey("control", "c2") != nil {
		t.Fatal("c2 was created without its relation")
	}
	c1 := w.byKey("control", "c1")
	if got := w.relations("mitigates"); !slices.Equal(got, []string{c1.ID + "->RSK-1"}) {
		t.Fatalf("mitigates = %v", got)
	}
}

// TestStaleAndPrune pins that a relation the source no longer names is
// reported and kept, or removed with prune.
func TestStaleAndPrune(t *testing.T) {
	src := `
entities:
  groups:
    type: control-group
    select: $.groups
    key: oscal_id
    properties: {oscal_id: id}
  controls:
    type: control
    select: $.controls
    key: oscal_id
    properties: {oscal_id: id}
    relations:
      - type: part-of
        target: groups
        key: group
        optional: true
        prune: PRUNE
`
	first := `{"groups": [{"id": "g1"}, {"id": "g2"}], "controls": [{"id": "c1", "group": "g1"}]}`
	moved := `{"groups": [{"id": "g1"}, {"id": "g2"}], "controls": [{"id": "c1", "group": "g2"}]}`
	for _, prune := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale", true: "prune"}[prune], func(t *testing.T) {
			w := newWorld(t)
			m := w.mapping(strings.Replace(src, "PRUNE", map[bool]string{false: "false", true: "true"}[prune], 1))
			w.run(m, FormatJSON, first)
			p := w.run(m, FormatJSON, moved)
			if len(p.RelationAdds) != 1 {
				t.Fatalf("adds = %+v", p.RelationAdds)
			}
			n := len(w.relations("part-of"))
			if prune && (len(p.RelationRemoves) != 1 || n != 1) {
				t.Fatalf("removes = %+v, relations = %d", p.RelationRemoves, n)
			}
			if !prune && (len(p.Stale) != 1 || n != 2) {
				t.Fatalf("stale = %+v, relations = %d", p.Stale, n)
			}
		})
	}
}

// TestMissingAndScope pins that an entity in scope the source dropped is
// reported and kept, that one outside the scope is not reported, and that
// a key held outside the scope is a record error.
func TestMissingAndScope(t *testing.T) {
	w := newWorld(t)
	w.create("control", "", map[string]any{"oscal_id": "x1", "catalog": "other"})
	m := w.mapping(`
entities:
  controls:
    type: control
    select: $.controls
    key: oscal_id
    scope: {catalog: example}
    properties: {oscal_id: id}
`)
	w.run(m, FormatJSON, `{"controls": [{"id": "c1"}, {"id": "c2"}]}`)
	p := w.plan(m, FormatJSON, `{"controls": [{"id": "c1"}, {"id": "x1"}]}`)
	if len(p.Missing) != 1 || p.Missing[0].Key != "c2" {
		t.Fatalf("missing = %+v", p.Missing)
	}
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "outside this import's scope") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	if w.controls() != 3 {
		t.Fatal("an entity was deleted")
	}
}

func TestDuplicateSourceKey(t *testing.T) {
	w := newWorld(t)
	p := w.plan(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1"}, {"id": "c1"}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "also used by the record at $.controls[0]") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	if !strings.HasPrefix(p.Errors[0].Location, "$.controls[1]") {
		t.Fatalf("location = %q", p.Errors[0].Location)
	}
}

func TestIDTemplate(t *testing.T) {
	w := newWorld(t)
	m := w.mapping(`
entities:
  docs:
    type: doc
    select: $.docs
    id: "doc-{{id}}"
    properties: {title: title}
`)
	w.run(m, FormatJSON, `{"docs": [{"id": "a1", "title": "T"}]}`)
	if _, err := w.st.GetEntity(w.ctx, entity.Ref{ID: "doc-a1"}); err != nil {
		t.Fatalf("doc-a1: %v", err)
	}
	if p := w.plan(m, FormatJSON, `{"docs": [{"id": "a1", "title": "T"}]}`); p.HasChanges() {
		t.Fatalf("second run: %+v", p)
	}
	bad := w.plan(m, FormatJSON, `{"docs": [{"id": "a b/c"}]}`)
	if len(bad.Errors) != 1 {
		t.Fatalf("an invalid id was accepted: %+v", bad)
	}
}

func TestHookErrors(t *testing.T) {
	cases := []struct {
		name, script, want string
	}{
		{"changes the key", `function transform(d) d.properties.oscal_id = "other" return d end`, "changed the key"},
		{"returns a number", `function transform(d) return 1 end`, "returned number"},
		{"unknown property", `function transform(d) d.properties.nope = "x" return d end`, "no such property"},
		{"unknown relation", `function transform(d) d.relations.nope = {"x"} return d end`, "no such relation"},
		{"raises", `function transform(d) error("boom") end`, "boom"},
		{"no network", `function transform(d) return require("socket") end`, "socket"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			w.files["scripts/h.lua"] = tc.script
			m := w.mapping(controlsMapping + "    hook: scripts/h.lua\n")
			p := w.plan(m, FormatJSON, `{"controls": [{"id": "c1"}]}`)
			if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, tc.want) {
				t.Fatalf("errors = %+v, want %q", p.Errors, tc.want)
			}
		})
	}
	t.Run("missing function", func(t *testing.T) {
		w := newWorld(t)
		w.files["scripts/h.lua"] = `x = 1`
		m := w.mapping(controlsMapping + "    hook: scripts/h.lua\n")
		root, _ := Decode([]byte(`{"controls": []}`), FormatJSON, m.Limits())
		_, err := Build(w.ctx, Deps{Meta: w.meta, Store: w.st, Gate: w.gate, Files: w.files}, m, root)
		if err == nil || !strings.Contains(err.Error(), "defines no function transform") {
			t.Fatalf("Build = %v", err)
		}
	})
}

func TestHookSetsValues(t *testing.T) {
	w := newWorld(t)
	w.files["scripts/h.lua"] = `
function transform(d, rec)
  d.properties.title = string.upper(rec.title)
  d.properties.weight = 2
  d.properties.tags = "one"
  d.content = "body"
  return d
end`
	m := w.mapping(controlsMapping + "    hook: scripts/h.lua\n")
	w.run(m, FormatJSON, `{"controls": [{"id": "c1", "title": "low"}]}`)
	got := w.byKey("control", "c1")
	if got.Properties["title"] != "LOW" || normalize(got.Properties["weight"]) != "2" ||
		normalize(got.Properties["tags"]) != "one" || got.Content != "body" {

		t.Fatalf("entity = %+v", got)
	}
}

// TestACL_WriteDenied pins that the writes go through the manager's ACL: a
// principal who may read but not create gets a failure per create.
func TestACL_WriteDenied(t *testing.T) {
	w := aclWorld(t, "roles:\n  reader:\n    read: [\"*\"]\nassignments:\n  alice: reader\n", "alice")
	p := w.plan(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1"}]}`)
	res, err := Apply(w.ctx, w.mgr, p, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 || len(res.Failures) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if w.controls() != 0 {
		t.Fatal("a denied create was written")
	}
}

func TestACL_WriteGranted(t *testing.T) {
	w := aclWorld(t, "roles:\n  editor:\n    read: [\"*\"]\n    create: [control]\n    update: [control]\n"+
		"assignments:\n  alice: editor\n", "alice")
	w.run(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1"}]}`)
	if w.controls() != 1 {
		t.Fatal("granted create not written")
	}
}

// TestHiddenRows pins that a stored row the gate hides is never named: a
// key it holds is a record error without its id, and it is not reported
// missing.
func TestHiddenRows(t *testing.T) {
	w := newWorld(t)
	secret := w.create("control", "", map[string]any{"oscal_id": "c1"})
	gone := w.create("control", "", map[string]any{"oscal_id": "c9"})
	w.gate = hideGate{inner: w.gate, hidden: map[string]bool{secret: true, gone: true}}
	p := w.plan(w.mapping(controlsMapping), FormatJSON, `{"controls": [{"id": "c1"}]}`)
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Message, "cannot read") {
		t.Fatalf("errors = %+v", p.Errors)
	}
	for _, e := range p.Errors {
		if strings.Contains(e.Message, secret) {
			t.Fatalf("error names a hidden row: %+v", e)
		}
	}
	if len(p.Missing) != 0 || len(p.Creates) != 0 {
		t.Fatalf("plan = %+v", p)
	}
}

func TestGateFailureFailsTheBuild(t *testing.T) {
	w := newWorld(t)
	w.create("control", "", map[string]any{"oscal_id": "c1"})
	w.gate = failGate{}
	m := w.mapping(controlsMapping)
	root, _ := Decode([]byte(`{"controls": [{"id": "c1"}]}`), FormatJSON, m.Limits())
	if _, err := Build(w.ctx, Deps{Meta: w.meta, Store: w.st, Gate: w.gate}, m, root); err == nil {
		t.Fatal("a gate failure was ignored")
	}
}

func TestMaxRecords(t *testing.T) {
	w := newWorld(t)
	m := w.mapping("max_records: 1\n" + controlsMapping)
	root, _ := Decode([]byte(`{"controls": [{"id": "a"}, {"id": "b"}]}`), FormatJSON, m.Limits())
	_, err := Build(w.ctx, Deps{Meta: w.meta, Store: w.st, Gate: w.gate}, m, root)
	if err == nil || !strings.Contains(err.Error(), "max_records is 1") {
		t.Fatalf("Build = %v", err)
	}
}

func TestYAMLAndCSV(t *testing.T) {
	cases := []struct {
		name, mapping, data string
		format              Format
	}{
		{"yaml", controlsMapping, "controls:\n  - id: c1\n    title: One\n  - id: c2\n    title: Two\n", FormatYAML},
		{"csv", strings.Replace(controlsMapping, "$.controls", "$", 1), "id,title\nc1,One\nc2,Two\n", FormatCSV},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			w.run(w.mapping(tc.mapping), tc.format, tc.data)
			if w.controls() != 2 || w.byKey("control", "c2").Properties["title"] != "Two" {
				t.Fatal("records not imported")
			}
		})
	}
}
