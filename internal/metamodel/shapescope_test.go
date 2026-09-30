package metamodel

import (
	"encoding/json"
	"testing"
)

// A relation's scope decides which tail its stored edges hang from, so it is
// part of the shape. These pin the projection, the hash and the classifier.

func scopeDoc(scope string) string {
	doc := "version: \"1\"\nentities:\n  guide:\n    label: Guide\n    id_prefix: GUIDE\n" +
		"    faces:\n      en: {}\n    properties:\n      title: {type: string}\n" +
		"relations:\n  cites:\n    label: Cites\n    from: [guide]\n    to: [guide]\n"
	if scope != "" {
		doc += "    scope: " + scope + "\n"
	}
	return doc
}

func TestShapeProjection_RelationScope(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  RelationScope
	}{
		{"implicit identity projects empty", "", ""},
		{"explicit identity projects empty", "identity", ""},
		{"content projects content", "content", ScopeContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shapeWithFaces(t, scopeDoc(tc.scope)).Relations["cites"].Scope
			if got != tc.want {
				t.Fatalf("Scope = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShapeProjection_ScopeHash(t *testing.T) {
	implicit := shapeWithFaces(t, scopeDoc("")).Hash()
	explicit := shapeWithFaces(t, scopeDoc("identity")).Hash()
	content := shapeWithFaces(t, scopeDoc("content")).Hash()
	if implicit != explicit {
		t.Error("spelling out scope: identity changes no stored edge; the hash must not move")
	}
	if implicit == content {
		t.Error("a content scope moves which tail edges hang from; the hash must move")
	}
}

func TestCompareShapes_RelationScopeChangeIsDrift(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     bool
	}{
		{"identity to content", "", "content", true},
		{"content to identity", "content", "identity", true},
		{"identity spellings", "", "identity", false},
		{"unchanged content", "content", "content", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := CompareShapes(shapeWithFaces(t, scopeDoc(tc.from)), shapeWithFaces(t, scopeDoc(tc.to)))
			var found bool
			for _, d := range r.Deltas {
				if d.Kind != "relation_scope_changed" {
					continue
				}
				found = true
				if d.Tier != TierDrift || d.Subject != "rel:cites" {
					t.Errorf("delta = %+v, want drift on rel:cites", d)
				}
			}
			if found != tc.want {
				t.Fatalf("relation_scope_changed reported = %t, want %t (deltas %+v)", found, tc.want, r.Deltas)
			}
		})
	}
}

// TestCompareShapes_OldRecordReportsNoScopeChange pins the upgrade path: a
// projection recorded before scopes joined the shape decodes a content-scoped
// relation as identity-scoped. Comparing it would report a scope change that
// never happened, so the comparison is skipped.
func TestCompareShapes_OldRecordReportsNoScopeChange(t *testing.T) {
	live := shapeWithFaces(t, scopeDoc("content"))
	raw, err := json.Marshal(live)
	if err != nil {
		t.Fatal(err)
	}
	var recorded ShapeProjection
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	recorded.RelationScopes = false
	rs := recorded.Relations["cites"]
	rs.Scope = ""
	recorded.Relations["cites"] = rs

	if r := CompareShapes(recorded, live); len(r.Deltas) != 0 {
		t.Fatalf("deltas = %+v, want none for a record that predates scopes", r.Deltas)
	}
}
