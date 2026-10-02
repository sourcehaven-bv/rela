package classification

import (
	"strings"
	"testing"
)

func testShape() Shape {
	return Shape{
		Entities: map[string]TypeShape{
			"person":     {Fields: []string{"title", "email", "body"}},
			"employment": {Fields: []string{"birth_date", "postcode", "gender", "salary", "body"}},
			"sick-leave": {Fields: []string{"diagnosis", "body"}},
			"company":    {Fields: []string{"title", "body"}},
		},
		Relations: map[string]RelationShape{
			"of":      {Ends: []Ends{{From: "employment", To: "person"}}},
			"watches": {Ends: []Ends{{From: "person", To: "company"}}},
			"reports": {Fields: []string{"note"}, Ends: []Ends{{From: "sick-leave", To: "person"}}},
		},
	}
}

func TestLint(t *testing.T) {
	type want struct{ code, path string }
	tests := []struct {
		name string
		in   string
		want []want
	}{
		{"worked example has gaps", workedExample, []want{
			{CodeMissingEntry, "assign.company"},
			{CodeMissingEntry, "assign_relations.reports"},
		}},
		{"complete file is clean", strings.Replace(workedExample, "subject:\n", `  company:
    title: none
    body: none
assign_relations:
  reports:
    note: [health]
subject:
`, 1), nil},
		{"needs-review and missing field", `
labels: {x: {}}
assign:
  person:
    title: needs-review
    body: none
`, []want{
			{CodeMissingEntry, "assign.employment"},
			{CodeMissingEntry, "assign.person.email"},
			{CodeNeedsReview, "assign.person.title"},
		}},
		{"stale type, field and override", `
assign:
  customer:
    title: none
  person:
    phone: none
subject:
  customer: true
subject_link:
  owns: false
subject_hops:
  owns: 2
`, []want{
			{CodeStaleEntry, "assign.customer"},
			{CodeStaleEntry, "assign.person.phone"},
			{CodeUnknownTarget, "subject.customer"},
			{CodeUnknownTarget, "subject_link.owns"},
			{CodeUnknownTarget, "subject_hops.owns"},
		}},
		{"hops on a relation that is not a subject link", `
subject_link:
  watches: false
subject_hops:
  watches: 2
`, []want{
			{CodeNoEffect, "subject_hops.watches"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := mustParse(t, tc.in)
			got := Lint(f, testShape())
			for _, w := range tc.want {
				found := false
				for _, is := range got {
					if is.Code == w.code && is.Path == w.path {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %s at %s in %v", w.code, w.path, got)
				}
			}
			if tc.want == nil && len(got) > 0 {
				t.Errorf("unexpected issues: %v", got)
			}
		})
	}
}

func TestLint_IssueLines(t *testing.T) {
	f := mustParse(t, "assign:\n  person:\n    title: needs-review\n    phone: none\n")
	for _, is := range Lint(f, testShape()) {
		switch is.Path {
		case "assign.person.title":
			if is.Line != 3 {
				t.Errorf("title line = %d, want 3", is.Line)
			}
		case "assign.person.phone":
			if is.Line != 4 {
				t.Errorf("phone line = %d, want 4", is.Line)
			}
		case "assign.person.email":
			if is.Line != 2 {
				t.Errorf("missing email line = %d, want the type's line 2", is.Line)
			}
		}
	}
}
