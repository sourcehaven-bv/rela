package metamodel

import (
	"strings"
	"testing"
)

const withinHead = `
version: "1.0"
entities:
  todo: {label: Todo, id_prefix: "TODO-", properties: {title: {type: string}}}
  project: {label: Project, id_prefix: "PRJ-", properties: {title: {type: string}}}
  area: {label: Area, id_prefix: "AREA-", properties: {title: {type: string}}}
  heading: {label: Heading, id_prefix: "HEAD-", properties: {title: {type: string}}}
relations:
`

const withinBase = `  in_project:
    label: in project
    from: [todo]
    to: [project]
    max_outgoing: 1
  has_heading:
    label: has heading
    from: [project]
    to: [heading]
    owning: true
`

// TestParse_RelationWithin pins the load-time rules for `within:` and
// `on_change:` (TKT-5GK80Q9). Every refusal names the relation.
func TestParse_RelationWithin(t *testing.T) {
	cases := []struct {
		name    string
		rels    string
		wantErr []string
	}{
		{
			name: "valid scoped relation loads",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: clear
`,
		},
		{
			name: "reject loads",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: reject
`,
		},
		{
			name: "on_change is required",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
`,
			wantErr: []string{`relation "under_heading"`, "requires on_change"},
		},
		{
			name: "invalid on_change",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: ignore
`,
			wantErr: []string{`relation "under_heading"`, `invalid on_change value "ignore"`},
		},
		{
			name: "on_change without within",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, "on_change is set without within"},
		},
		{
			name: "unknown scope relation",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: nope
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, `unknown relation "nope"`},
		},
		{
			name: "scope relation must be single-valued",
			rels: `  in_project:
    label: in project
    from: [todo]
    to: [project]
  has_heading:
    label: has heading
    from: [project]
    to: [heading]
    owning: true
  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, "max_outgoing: 1"},
		},
		{
			name: "scope relation must cover from types",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo, area]
    to: [heading]
    within: in_project
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, `does not include "area"`},
		},
		{
			name: "target without owner",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading, area]
    within: in_project
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, `type "area" cannot resolve to an owner`},
		},
		{
			name: "owner from another type does not count",
			rels: `  in_project:
    label: in project
    from: [todo]
    to: [project]
    max_outgoing: 1
  area_heading:
    label: area heading
    from: [area]
    to: [heading]
    owning: true
  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: clear
`,
			wantErr: []string{`type "heading" cannot resolve to an owner`},
		},
		{
			name: "two owning relations are ambiguous",
			rels: withinBase + `  also_heading:
    label: also heading
    from: [project]
    to: [heading]
    owning: true
  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: clear
`,
			wantErr: []string{`type "heading" cannot resolve to an owner`, "also_heading, has_heading"},
		},
		{
			name: "scoped relation may not be owning",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    owning: true
    within: in_project
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, "combined with owning"},
		},
		{
			name: "scoped relation may not be symmetric",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    symmetric: true
    within: in_project
    on_change: clear
`,
			wantErr: []string{`relation "under_heading"`, "combined with symmetric"},
		},
		{
			name: "self reference is a cycle",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: under_heading
    on_change: clear
`,
			wantErr: []string{"within forms a cycle: under_heading -> under_heading"},
		},
		{
			name: "two-relation cycle",
			rels: withinBase + `  a:
    label: a
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: b
    on_change: clear
  b:
    label: b
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: a
    on_change: clear
`,
			wantErr: []string{"within forms a cycle: a -> b -> a"},
		},
		{
			name: "three-relation cycle",
			rels: withinBase + `  a:
    label: a
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: b
    on_change: clear
  b:
    label: b
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: c
    on_change: clear
  c:
    label: c
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: a
    on_change: clear
`,
			wantErr: []string{"within forms a cycle: a -> b -> c -> a"},
		},
		{
			name: "scope relation may not be scoped itself",
			rels: withinBase + `  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: in_project
    on_change: clear
  sub_heading:
    label: sub heading
    from: [todo]
    to: [heading]
    within: under_heading
    on_change: clear
`,
			wantErr: []string{`relation "sub_heading"`, "one level deep"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(withinHead + tc.rels))
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if owner, ok := ScopeOwner(m, "under_heading", "heading"); !ok || owner != "has_heading" {
					t.Errorf("ScopeOwner = %q, %v; want has_heading, true", owner, ok)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error, schema loaded: %+v", m.Relations)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}

// TestParse_RelationWithin_CycleReportedOnce guards the dedup: one cycle is
// one problem, however many relations are on it.
func TestParse_RelationWithin_CycleReportedOnce(t *testing.T) {
	rels := withinBase + `  a:
    label: a
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: b
    on_change: clear
  b:
    label: b
    from: [todo]
    to: [heading]
    max_outgoing: 1
    within: a
    on_change: clear
`
	_, err := Parse([]byte(withinHead + rels))
	if err == nil {
		t.Fatal("expected an error")
	}
	if n := strings.Count(err.Error(), "forms a cycle"); n != 1 {
		t.Errorf("cycle reported %d times, want 1: %v", n, err)
	}
}

func TestScopeOwner_NotScoped(t *testing.T) {
	m, err := Parse([]byte(withinHead + withinBase))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ rel, typ string }{
		{"in_project", "project"}, {"missing", "heading"},
	} {
		if _, ok := ScopeOwner(m, tc.rel, tc.typ); ok {
			t.Errorf("ScopeOwner(%s, %s) = ok, want not scoped", tc.rel, tc.typ)
		}
	}
	if _, ok := ScopeOwner(nil, "x", "y"); ok {
		t.Error("ScopeOwner(nil) = ok")
	}
	if ScopeChange("x").IsValid() || !ScopeChangeClear.IsValid() {
		t.Error("ScopeChange.IsValid wrong")
	}
}
