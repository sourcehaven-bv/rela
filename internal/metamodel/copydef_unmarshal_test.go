package metamodel

import (
	"reflect"
	"strings"
	"testing"
)

// TestCopyDef_UnmarshalCoversEveryField is the guard [CopyDef.UnmarshalYAML]'s
// comment promises.
//
// # The bug it exists for, which actually happened
//
// CopyDef has a custom UnmarshalYAML (to accept `fields: all` as a scalar),
// and that method decodes into a private shadow struct listing the fields it
// knows about. A field added to CopyDef but NOT to the shadow struct is
// SILENTLY DROPPED at load: no error, no warning, just a zero value. `label:`
// was added and ignored exactly this way, and it surfaced only because a test
// asserted on the parsed value.
//
// That is a check whose failure mode is silence: an operator writes `label:
// Publish` in schema.yaml, the config loads without complaint, and the button
// renders with the wrong text. Nothing anywhere reports a problem.
//
// # What this asserts
//
// Every yaml-tagged field on CopyDef round-trips through a real Parse. A new
// field with a `yaml:` tag fails here until the shadow struct learns it.
//
// It parses REAL YAML rather than comparing the two structs by reflection
// alone: a shadow struct could list a field and still fail to assign it (the
// method copies fields explicitly), and only an end-to-end parse catches that.
func TestCopyDef_UnmarshalCoversEveryField(t *testing.T) {
	t.Parallel()

	// One YAML document exercising every yaml-bearing field of CopyDef.
	const doc = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    pointers:
      draft: {default: true}
      published: {}
    properties:
      title: {type: string}
relations:
  # CONTENT-scoped: only content-scoped relations may be copied — an identity
  # edge is shared by every face, so copying it would duplicate an edge that
  # may confer roles.
  mentions:
    scope: content
    from: [page]
    to: [page]
copies:
  everything:
    from: page@draft
    to: page@published
    label: Publish It
    fields:
      title: "{{new.title}}"
    relations:
      mentions: merge
    guard:
      permission: promote-page
`
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, ok := m.Copies["everything"]
	if !ok {
		t.Fatal("copy definition did not load")
	}

	// Each yaml field, asserted non-zero. A field the shadow struct forgets
	// arrives as its zero value, which is exactly what this catches.
	checks := map[string]bool{
		"From":      def.From == "page@draft",
		"To":        def.To == "page@published",
		"Label":     def.Label == "Publish It",
		"Fields":    def.Fields["title"] == "{{new.title}}",
		"Relations": def.Relations["mentions"] == "merge",
		// Guard.When is deliberately NOT exercised: `guard.when` is refused at
		// load ("not implemented yet — remove it, or the copy would run
		// without evaluating the condition you wrote"), so a document setting
		// it cannot parse. Permission alone proves the Guard struct arrives.
		"Guard": def.Guard.Permission == "promote-page",
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("CopyDef.%s did not survive UnmarshalYAML — a field missing "+
				"from the method's shadow `raw` struct is silently dropped at "+
				"load, with no error. Add it there and to the explicit "+
				"assignment below it.", name)
		}
	}

	assertEveryYAMLFieldChecked(t, CopyDef{}, checks)
}

// assertEveryYAMLFieldChecked fails when a yaml-tagged field of v is absent
// from checks.
//
// Without this, the surviving-field loop above passes VACUOUSLY for any field
// nobody thought to add — which is the same "the check cannot report what it
// does not know about" shape as the enumeration bugs this epic has hit in the
// route guard and in the unmarshaler itself. The reflection makes the
// enumeration self-checking.
//
// A `yaml:"-"` field is skipped: it is deliberately not decoded (CopyDef's
// AllFields is set from the `fields` scalar by hand).
func assertEveryYAMLFieldChecked(t *testing.T, v any, checks map[string]bool) {
	t.Helper()
	var missing []string
	rt := reflect.TypeOf(v)
	for f := range rt.Fields() {
		tag := f.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		if _, covered := checks[f.Name]; !covered {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s gained yaml-tagged field(s) %s that this test does not "+
			"exercise. Add them to the YAML document and the checks, or a field "+
			"silently dropped by UnmarshalYAML would pass unnoticed.",
			rt.Name(), strings.Join(missing, ", "))
	}
}

// TestWorldDef_UnmarshalCoversEveryField is the same guard on the OTHER type
// in this package with the vulnerable shape.
//
// # Why WorldDef, and why only these two
//
// A sweep of internal/metamodel found ten custom UnmarshalYAML methods. Only
// two use the shadow-struct-plus-explicit-copy shape that can silently drop a
// field: CopyDef (which was buggy — `label:` vanished) and WorldDef. The rest
// (ScanPolicy, ACLBypass, InverseDef, HeaderCheck, StringOrSlice, oneOrMany)
// are scalar-or-sequence coercions with no field enumeration to fall out of
// sync, so they cannot exhibit this failure and are deliberately not covered
// here.
//
// WorldDef is CLEAN today — all four of its yaml fields are copied. It is
// guarded anyway because it is the type this epic keeps extending, and
// `otherwise:` in particular decides resolution semantics: a version of the
// CopyDef bug that dropped it would silently change which face every reader
// sees, with the config loading without complaint.
//
// "Clean by luck" and "clean by construction" look identical until someone
// adds a field.
func TestWorldDef_UnmarshalCoversEveryField(t *testing.T) {
	t.Parallel()

	const doc = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    pointers:
      draft: {default: true}
      review: {}
      published: {}
    properties:
      title: {type: string}
worlds:
  everything:
    select: [review, published]
    overrides:
      page: [published]
    otherwise: default
    edits: draft
`
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, ok := m.Worlds["everything"]
	if !ok {
		t.Fatal("world definition did not load")
	}

	checks := map[string]bool{
		"Select":    len(def.Select) == 2 && def.Select[0] == "review" && def.Select[1] == "published",
		"Overrides": len(def.Overrides["page"]) == 1 && def.Overrides["page"][0] == "published",
		"Otherwise": string(def.Otherwise) == "default",
		"Edits":     def.Edits == "draft",
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("WorldDef.%s did not survive UnmarshalYAML — a field missing "+
				"from the method's shadow struct is silently dropped at load, "+
				"with no error. For a world, that silently changes which face "+
				"every reader sees.", name)
		}
	}

	assertEveryYAMLFieldChecked(t, WorldDef{}, checks)
}
