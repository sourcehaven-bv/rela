package classification

import (
	"slices"
	"strings"
	"testing"
)

func smallShape() Shape {
	return Shape{
		Entities: map[string]TypeShape{
			"person": {Fields: []string{"title", "email", "body"}},
		},
		Relations: map[string]RelationShape{
			"knows": {Ends: []Ends{{From: "person", To: "person"}}},
		},
	}
}

func runSync(t *testing.T, in string, shape Shape, renames []Rename) SyncResult {
	t.Helper()
	res, issues := Sync([]byte(in), shape, renames)
	if len(issues) > 0 {
		t.Fatalf("sync issues: %v", issues)
	}
	if _, parseIssues := Parse(res.Output); len(parseIssues) > 0 {
		t.Fatalf("sync output does not parse: %v\n%s", parseIssues, res.Output)
	}
	again, issues := Sync(res.Output, shape, renames)
	if len(issues) > 0 || again.Changed {
		t.Fatalf("sync is not idempotent: changed=%v issues=%v\n%s", again.Changed, issues, again.Output)
	}
	return res
}

func TestSync_Fresh(t *testing.T) {
	res := runSync(t, "", smallShape(), nil)
	want := `# Data classification: describes what data each field holds. It changes no
# behavior and is not access control. See docs/classification.md.
labels: {}
assign:
  person:
    title: needs-review
    email: needs-review
    body: needs-review
`
	if string(res.Output) != want {
		t.Errorf("output:\n%s\nwant:\n%s", res.Output, want)
	}
	if !res.Changed || len(res.Added) != 3 {
		t.Errorf("result = %+v", res)
	}
	f := mustParse(t, string(res.Output))
	if got := Lint(f, smallShape()); len(got) != 3 || got[0].Code != CodeNeedsReview {
		t.Errorf("lint after fresh sync = %v", got)
	}
}

func TestSync_CommentOnlyFileKeepsComments(t *testing.T) {
	res := runSync(t, "# owned by the privacy team\n\n# ask before editing\n", smallShape(), nil)
	want := "# owned by the privacy team\n# ask before editing\n\n# Data classification:"
	if !strings.HasPrefix(string(res.Output), want) {
		t.Errorf("output:\n%s\nwant prefix:\n%s", res.Output, want)
	}
	mustParse(t, string(res.Output))
}

func TestSync_KeepsCommentsAndOrder(t *testing.T) {
	in := `# Owned by the privacy team.
labels:
  contact: { role: direct-identifier } # emails and phones
assign:
  person:
    # the display name
    title: none
    body: none
`
	res := runSync(t, in, smallShape(), nil)
	want := `# Owned by the privacy team.
labels:
  contact: {role: direct-identifier} # emails and phones
assign:
  person:
    # the display name
    title: none
    email: needs-review
    body: none
`
	if string(res.Output) != want {
		t.Errorf("output:\n%s\nwant:\n%s", res.Output, want)
	}
	if !slices.Equal(res.Added, []string{"assign.person.email"}) {
		t.Errorf("added = %v", res.Added)
	}
}

func TestSync_UpToDateIsByteIdentical(t *testing.T) {
	in := "labels:   {}\nassign:\n  person:\n    title:   none\n    email: none\n    body: none\n"
	res, issues := Sync([]byte(in), smallShape(), nil)
	if len(issues) > 0 || res.Changed || string(res.Output) != in {
		t.Errorf("changed=%v issues=%v output=%q", res.Changed, issues, res.Output)
	}
}

func TestSync_RefusesBrokenFile(t *testing.T) {
	in := "labels:\n  x: {}\n  x: {}\n"
	res, issues := Sync([]byte(in), smallShape(), nil)
	if len(issues) == 0 || res.Changed || string(res.Output) != in {
		t.Errorf("want refusal, got changed=%v issues=%v", res.Changed, issues)
	}
}

func TestSync_Renames(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		shape     Shape
		renames   []Rename
		contains  []string
		moved     []string
		stale     []string
		conflicts []string
	}{
		{
			name: "property rename",
			in:   "assign:\n  person:\n    name: [n]\n    email: none\n    body: none\nlabels: {n: {}}\n",
			renames: []Rename{
				{Kind: RenameProperty, Owner: "person", From: "name", To: "title"},
			},
			contains: []string{"title: [n]"},
			moved:    []string{"assign.person.name -> title"},
		},
		{
			name: "type rename then property rename under the new name",
			in:   "assign:\n  human:\n    name: none\n    email: none\n    body: none\n",
			renames: []Rename{
				{Kind: RenameEntityType, From: "human", To: "person"},
				{Kind: RenameProperty, Owner: "person", From: "name", To: "title"},
			},
			contains: []string{"person:\n    title: none"},
			moved:    []string{"assign.human -> person", "assign.person.name -> title"},
		},
		{
			name: "property rename under the old type name",
			in:   "assign:\n  human:\n    name: none\n    email: none\n    body: none\n",
			renames: []Rename{
				{Kind: RenameProperty, Owner: "human", From: "name", To: "title"},
				{Kind: RenameEntityType, From: "human", To: "person"},
			},
			contains: []string{"person:\n    title: none"},
			moved:    []string{"assign.human -> person", "assign.person.name -> title"},
		},
		{
			name: "rename chain",
			in:   "assign:\n  a:\n    title: none\n    email: none\n    body: none\n",
			renames: []Rename{
				{Kind: RenameEntityType, From: "a", To: "b"},
				{Kind: RenameEntityType, From: "b", To: "person"},
			},
			contains: []string{"person:\n    title: none"},
			moved:    []string{"assign.a -> person"},
		},
		{
			name: "old name reused by a new type keeps its entry",
			in:   "assign:\n  person:\n    title: [n]\n    email: none\n    body: none\nlabels: {n: {}}\n",
			shape: Shape{Entities: map[string]TypeShape{
				"person":   {Fields: []string{"title", "email", "body"}},
				"customer": {Fields: []string{"title"}},
			}},
			renames: []Rename{
				{Kind: RenameEntityType, From: "person", To: "customer"},
			},
			contains: []string{"person:\n    title: [n]", "customer:\n    title: needs-review"},
		},
		{
			name: "rename target already has an entry",
			in:   "assign:\n  human:\n    title: [n]\n  person:\n    title: none\n    email: none\n    body: none\nlabels: {n: {}}\n",
			renames: []Rename{
				{Kind: RenameEntityType, From: "human", To: "person"},
			},
			conflicts: []string{"assign.human -> person: person already has an entry"},
		},
		{
			name:     "removed type is stale, not deleted",
			in:       "assign:\n  gone:\n    title: none\n  person:\n    title: none\n    email: none\n    body: none\n",
			stale:    []string{"assign.gone"},
			contains: []string{"gone:\n    title: none"},
		},
		{
			name:     "rename to a type the schema no longer has",
			in:       "assign:\n  a:\n    title: none\n  person:\n    title: none\n    email: none\n    body: none\n",
			renames:  []Rename{{Kind: RenameEntityType, From: "a", To: "b"}},
			stale:    []string{"assign.a"},
			contains: []string{"a:\n    title: none"},
		},
		{
			name: "override keys follow renames",
			in: "assign:\n  person:\n    title: none\n    email: none\n    body: none\n" +
				"subject:\n  human: true\nsubject_link:\n  befriends: false\nsubject_hops:\n  befriends: 2\n",
			renames: []Rename{
				{Kind: RenameEntityType, From: "human", To: "person"},
				{Kind: RenameRelationType, From: "befriends", To: "knows"},
			},
			contains: []string{"subject:\n  person: true", "subject_link:\n  knows: false", "subject_hops:\n  knows: 2"},
			moved: []string{"subject.human -> person", "subject_link.befriends -> knows",
				"subject_hops.befriends -> knows"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shape := tc.shape
			if shape.Entities == nil {
				shape = smallShape()
			}
			res := runSync(t, tc.in, shape, tc.renames)
			for _, c := range tc.contains {
				if !strings.Contains(string(res.Output), c) {
					t.Errorf("output lacks %q:\n%s", c, res.Output)
				}
			}
			if !slices.Equal(res.Moved, tc.moved) {
				t.Errorf("moved = %v, want %v", res.Moved, tc.moved)
			}
			if !slices.Equal(res.Stale, tc.stale) {
				t.Errorf("stale = %v, want %v", res.Stale, tc.stale)
			}
			if !slices.Equal(res.Conflicts, tc.conflicts) {
				t.Errorf("conflicts = %v, want %v", res.Conflicts, tc.conflicts)
			}
		})
	}
}
