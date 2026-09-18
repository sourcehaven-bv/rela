package metamodel

import (
	"strings"
	"testing"
)

// TestQueryScopes_Parse pins that `query_scopes:` reaches the EntityDef as
// raw source. The metamodel deliberately does not compile it (internal/scopes
// does), so what this asserts is that the strings survive the load unaltered.
func TestQueryScopes_Parse(t *testing.T) {
	m, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties:
      status: {type: string}
      toegewezen_aan: {type: string}
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      actief: "entity.status ~= 'gereed'"
      mijn: "is_current_user(entity.toegewezen_aan)"
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, ok := m.GetEntityDef("taak")
	if !ok {
		t.Fatal("taak not found")
	}
	if got, want := len(def.QueryScopes), 3; got != want {
		t.Fatalf("QueryScopes has %d entries, want %d", got, want)
	}
	if got, want := def.QueryScopes["mijn"], "is_current_user(entity.toegewezen_aan)"; got != want {
		t.Errorf("QueryScopes[mijn] = %q, want %q", got, want)
	}
}

// TestQueryScopes_AbsentIsUnchanged pins the byte-identical claim in the
// EntityDef doc: a type that declares no scopes carries an empty map, not a
// populated one with an implicit entry.
func TestQueryScopes_AbsentIsUnchanged(t *testing.T) {
	m, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	def, _ := m.GetEntityDef("taak")
	if len(def.QueryScopes) != 0 {
		t.Errorf("QueryScopes = %v, want empty", def.QueryScopes)
	}
}

func TestQueryScopes_Rejects(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  string
	}{
		{
			name:  "the reserved name all",
			scope: `      all: "true"`,
			want:  `is reserved`,
		},
		{
			name:  "the reserved name all, case-folded",
			scope: `      All: "true"`,
			want:  `is reserved`,
		},
		{
			name:  "an empty expression",
			scope: `      actief: ""`,
			want:  `has an empty expression`,
		},
		{
			name:  "a whitespace-only expression",
			scope: `      actief: "   "`,
			want:  `has an empty expression`,
		},
		{
			name:  "a name with a space",
			scope: `      "recent done": "true"`,
			want:  `is not a valid name`,
		},
		{
			name:  "a name with a slash",
			scope: `      "a/b": "true"`,
			want:  `is not a valid name`,
		},
		{
			name:  "a name starting with a digit",
			scope: `      "2fast": "true"`,
			want:  `is not a valid name`,
		},
		{
			name:  "a name with a doubled separator",
			scope: `      "a__b": "true"`,
			want:  `is not a valid name`,
		},
		{
			name:  "a name with a trailing separator",
			scope: `      "a-": "true"`,
			want:  `is not a valid name`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
` + tc.scope + "\n"))
			if err == nil {
				t.Fatalf("Parse succeeded, want an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestQueryScopes_Accepts pins the names an operator will actually reach for.
// The grammar is an allowlist, so a false rejection is the likely defect.
func TestQueryScopes_Accepts(t *testing.T) {
	for _, name := range []string{
		"default", "actief", "recent_done", "recent-done", "a", "a1",
		"mijn_open_taken", "x9-y8_z7",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
      ` + name + `: "entity.status ~= 'x'"
`))
			if err != nil {
				t.Errorf("Parse rejected valid scope name %q: %v", name, err)
			}
		})
	}
}

// TestQueryScopes_CollectsEveryProblem pins the loader's collect-then-report
// discipline: an operator fixing a schema should see the whole list, not the
// first problem and then another run.
func TestQueryScopes_CollectsEveryProblem(t *testing.T) {
	_, err := Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
      all: "true"
      "bad name": "true"
      leeg: ""
`))
	if err == nil {
		t.Fatal("Parse succeeded, want errors")
	}
	for _, want := range []string{"is reserved", "is not a valid name", "has an empty expression"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q; got:\n%v", want, err)
		}
	}
}

// TestQueryScopes_NotInShapeProjection pins that editing a query scope is not
// a data-shape change. ShapeProjection's hash gates data migration: if scopes
// were included, adding one would demand a migration for a change that alters
// no stored bytes. Both projections are allowlists, so this holds by
// construction — the test exists to keep it that way.
func TestQueryScopes_NotInShapeProjection(t *testing.T) {
	const base = `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
`
	without, err := Parse([]byte(base))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	with, err := Parse([]byte(base + `    query_scopes:
      actief: "entity.status ~= 'gereed'"
`))
	if err != nil {
		t.Fatalf("Parse with scopes: %v", err)
	}
	if a, b := without.ShapeProjection().Hash(), with.ShapeProjection().Hash(); a != b {
		t.Errorf("declaring a query scope changed the shape hash (%s -> %s); "+
			"a scope alters no stored data and must not demand a migration", a, b)
	}
	if a, b := without.RenderProjection().Hash(), with.RenderProjection().Hash(); a != b {
		t.Errorf("declaring a query scope changed the render hash (%s -> %s); "+
			"version rendering does not depend on scopes", a, b)
	}
}
