package metamodel

import (
	"strings"
	"testing"
)

// TestParse_RelationOwning pins the load-time rules for `owning:`
// (TKT-QO14GB): each refused combination names the relation, and a plain
// owning relation (including one whose ends are the same type) loads.
func TestParse_RelationOwning(t *testing.T) {
	const head = `
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title: {type: string}
  page:
    label: Page
    id_prefix: "PAGE-"
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
relations:
`
	cases := []struct {
		name    string
		rel     string
		wantErr []string
	}{
		{
			name: "same type on both ends loads",
			rel: `  subtask:
    label: subtask
    from: [task]
    to: [task]
    owning: true
`,
		},
		{
			name: "max_incoming of one loads",
			rel: `  subtask:
    label: subtask
    from: [task]
    to: [task]
    owning: true
    max_incoming: 1
`,
		},
		{
			name: "symmetric is refused",
			rel: `  subtask:
    label: subtask
    from: [task]
    to: [task]
    owning: true
    symmetric: true
`,
			wantErr: []string{`relation "subtask"`, "symmetric"},
		},
		{
			name: "max_incoming above one is refused",
			rel: `  subtask:
    label: subtask
    from: [task]
    to: [task]
    owning: true
    max_incoming: 2
`,
			wantErr: []string{`relation "subtask"`, "max_incoming is 2"},
		},
		{
			name: "faced source is refused",
			rel: `  subtask:
    label: subtask
    from: [page]
    to: [task]
    owning: true
`,
			wantErr: []string{`relation "subtask"`, `type "page"`},
		},
		{
			name: "faced target is refused",
			rel: `  subtask:
    label: subtask
    from: [task]
    to: [page]
    owning: true
`,
			wantErr: []string{`relation "subtask"`, `type "page"`},
		},
		{
			name: "faced type on a non-owning relation loads",
			rel: `  mentions:
    label: mentions
    from: [page]
    to: [task]
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(head + tc.rel))
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
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

// TestParse_RelationOwning_FacedTypeOnBothEndsReportedOnce guards the
// dedup: a faced type named on both ends is one problem, not two.
func TestParse_RelationOwning_FacedTypeOnBothEndsReportedOnce(t *testing.T) {
	const doc = `
version: "1.0"
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    faces:
      draft: {}
    properties:
      title: {type: string}
relations:
  part-of:
    label: part of
    from: [page]
    to: [page]
    owning: true
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("expected an error")
	}
	if n := strings.Count(err.Error(), `type "page", which declares faces`); n != 1 {
		t.Errorf("faced type reported %d times, want 1: %v", n, err)
	}
}
