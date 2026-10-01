package classification

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// workedShape is the plan's worked example: a person, their employment and
// sick leave, a company they watch, and a person-to-person relation.
func workedShape() Shape {
	return Shape{
		Entities: map[string]TypeShape{
			"person":     {Fields: []string{"title", "email", "body"}, OpaqueID: true},
			"employment": {Fields: []string{"birth_date", "postcode", "gender", "salary", "body"}, OpaqueID: true},
			"sick-leave": {Fields: []string{"diagnosis", "body"}, OpaqueID: true},
			"company":    {Fields: []string{"title", "body"}},
		},
		Relations: map[string]RelationShape{
			"of":      {Ends: []Ends{{From: "employment", To: "person"}, {From: "sick-leave", To: "person"}}},
			"manages": {Fields: []string{"note"}, Ends: []Ends{{From: "person", To: "person"}}},
			"watches": {Ends: []Ends{{From: "person", To: "company"}}},
		},
	}
}

// workedFile is the worked example without its overrides.
const workedFile = `
labels:
  name:       { role: direct-identifier }
  contact:    { role: direct-identifier }
  birth-date: { role: quasi-identifier }
  postcode:   { role: quasi-identifier }
  gender:     { role: quasi-identifier }
  salary:     { role: attribute }
  health:     { role: attribute }
  identified-person:
    role: direct-identifier
    when:
      any_of:
        - all_of: ["@direct-identifier", "@quasi-identifier"]
        - count: { of: "@quasi-identifier", min: 3 }
  identified-health:
    role: attribute
    when:
      scope: subject
      all_of: ["@direct-identifier", health]
assign:
  person:
    title: [name]
    email: [contact]
    body: none
  employment:
    birth_date: [birth-date]
    postcode: [postcode]
    gender: [gender]
    salary: [salary]
    body: none
  sick-leave:
    diagnosis: [health]
    body: needs-review
  company:
    title: none
    body: none
assign_relations:
  manages:
    note: none
`

func subjectTypes(ss []Subject) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Type)
	}
	return out
}

func TestSubjects(t *testing.T) {
	tests := []struct {
		name      string
		overrides string
		want      []string
		reasons   map[string]string
	}{
		{"inferred", "", []string{"employment", "person"}, map[string]string{
			"person":     "person.title has name (direct-identifier)",
			"employment": "derived identified-person (direct-identifier) from employment.birth_date, employment.gender, employment.postcode",
		}},
		{"override removes", "subject:\n  employment: false\n", []string{"person"}, nil},
		{"override adds", "subject:\n  company: true\n", []string{"company", "employment", "person"},
			map[string]string{"company": "subject override"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := mustParse(t, workedFile+tc.overrides)
			got := f.Subjects(workedShape())
			if !slices.Equal(subjectTypes(got), tc.want) {
				t.Fatalf("subjects = %v, want %v", got, tc.want)
			}
			for _, s := range got {
				if want, ok := tc.reasons[s.Type]; ok && s.Reason != want {
					t.Errorf("%s reason = %q, want %q", s.Type, s.Reason, want)
				}
			}
		})
	}
}

func profileOf(ps []Profile, subject string) *Profile {
	for i := range ps {
		if ps[i].Subject == subject {
			return &ps[i]
		}
	}
	return nil
}

func reachedTypes(p *Profile) []string {
	var out []string
	for _, r := range p.Reached {
		out = append(out, r.Type)
	}
	return out
}

func TestProfiles(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		f := mustParse(t, workedFile)
		ps := f.Profiles(workedShape())
		person := profileOf(ps, "person")
		// employment is itself a subject, so it is not part of person's
		// profile; sick-leave and company are reached in one hop; manages
		// connects two persons and is never followed.
		if got := reachedTypes(person); !slices.Equal(got, []string{"company", "sick-leave"}) {
			t.Errorf("person reached = %v", got)
		}
		if !slices.Equal(person.Relations, []string{"of", "watches"}) {
			t.Errorf("person relations = %v", person.Relations)
		}
		if got := reachedTypes(profileOf(ps, "employment")); len(got) != 0 {
			t.Errorf("employment reached = %v; person is a subject and stops traversal", got)
		}
	})
	t.Run("overrides", func(t *testing.T) {
		f := mustParse(t, workedFile+"subject:\n  employment: false\nsubject_link:\n  watches: false\n")
		ps := f.Profiles(workedShape())
		if len(ps) != 1 {
			t.Fatalf("profiles = %v", ps)
		}
		if got := reachedTypes(&ps[0]); !slices.Equal(got, []string{"employment", "sick-leave"}) {
			t.Errorf("reached = %v", got)
		}
		want := []Hop{{Relation: "of", Outgoing: false, Type: "employment"}}
		if got := ps[0].Reached[0].Path; !slices.Equal(got, want) {
			t.Errorf("path = %v, want %v", got, want)
		}
	})
}

func TestProfiles_Hops(t *testing.T) {
	shape := Shape{
		Entities: map[string]TypeShape{
			"person": {Fields: []string{"name"}},
			"a":      {Fields: []string{"x"}},
			"b":      {Fields: []string{"x"}},
			"c":      {Fields: []string{"x"}},
		},
		Relations: map[string]RelationShape{
			"r1": {Ends: []Ends{{From: "person", To: "a"}}},
			"r2": {Ends: []Ends{{From: "a", To: "b"}}},
			"r3": {Ends: []Ends{{From: "b", To: "c"}}},
		},
	}
	base := "labels: {n: {role: direct-identifier}}\nassign:\n  person:\n    name: [n]\n"
	tests := []struct {
		name      string
		overrides string
		want      []string
	}{
		{"one hop by default", "", []string{"a"}},
		{"r2 is not a subject link", "subject_hops:\n  r2: 2\n", []string{"a"}},
		{"link and hops", "subject_link:\n  r2: true\nsubject_hops:\n  r2: 2\n", []string{"a", "b"}},
		{"hop limit is per relation", "subject_link:\n  r2: true\n  r3: true\nsubject_hops:\n  r2: 2\n",
			[]string{"a", "b"}},
		{"three hops", "subject_link:\n  r2: true\n  r3: true\nsubject_hops:\n  r2: 2\n  r3: 3\n",
			[]string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := mustParse(t, base+tc.overrides)
			ps := f.Profiles(shape)
			if got := reachedTypes(&ps[0]); !slices.Equal(got, tc.want) {
				t.Errorf("reached = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProfiles_Truncated(t *testing.T) {
	shape := Shape{
		Entities:  map[string]TypeShape{"person": {Fields: []string{"name"}}},
		Relations: map[string]RelationShape{},
	}
	var ends []Ends
	for i := range MaxProfileTypes + 5 {
		name := "t" + itoa3(i)
		shape.Entities[name] = TypeShape{}
		ends = append(ends, Ends{From: name, To: "person"})
	}
	shape.Relations["of"] = RelationShape{Ends: ends}
	f := mustParse(t, "labels: {n: {role: direct-identifier}}\nassign:\n  person:\n    name: [n]\n")
	p := f.Profiles(shape)[0]
	if !p.Truncated || len(p.Reached) != MaxProfileTypes-1 {
		t.Errorf("truncated=%v reached=%d", p.Truncated, len(p.Reached))
	}
}

func TestBuildReport_WorkedExample(t *testing.T) {
	f := mustParse(t, workedFile)
	rep := BuildReport(f, workedShape())

	byType := map[string]TypeReport{}
	for _, tr := range rep.Types {
		byType[tr.Type] = tr
	}
	emp := byType["employment"]
	if len(emp.Derived) != 1 || emp.Derived[0].Label != "identified-person" ||
		!slices.Equal(emp.Derived[0].Fields, []string{
			"employment.birth_date", "employment.gender", "employment.postcode",
		}) {

		t.Errorf("employment derived = %+v", emp.Derived)
	}
	if byType["company"].Subject != "" || byType["person"].Subject == "" {
		t.Errorf("subjects: company=%q person=%q", byType["company"].Subject, byType["person"].Subject)
	}
	if got := byType["sick-leave"].Fields[1]; got.State != StateNeedsReview {
		t.Errorf("sick-leave.body = %+v", got)
	}

	var person SubjectReport
	for _, s := range rep.Subjects {
		if s.Subject == "person" {
			person = s
		}
	}
	if len(person.Derived) != 1 || person.Derived[0].Label != "identified-health" ||
		!slices.Equal(person.Derived[0].Fields, []string{"person.email", "person.title", "sick-leave.diagnosis"}) {

		t.Errorf("person subject derived = %+v", person.Derived)
	}

	links := map[string]bool{}
	for _, r := range rep.Relations {
		links[r.Relation] = r.SubjectLink
	}
	if !links["of"] || !links["manages"] || !links["watches"] {
		t.Errorf("subject links = %v", links)
	}
}

func TestBuildReport_SubjectLinkOverride(t *testing.T) {
	f := mustParse(t, workedFile+"subject:\n  employment: false\nsubject_link:\n  of: false\n")
	rep := BuildReport(f, workedShape())
	for _, s := range rep.Subjects {
		if len(s.Derived) != 0 {
			t.Errorf("%s derived = %+v; without the of link, health is not linked to the person", s.Subject, s.Derived)
		}
	}
}

func TestBuildReport_RecordLabelInSubjectScope(t *testing.T) {
	// The person is identified only through three quasi-identifiers on the
	// employment record; the subject-scope rule must still see it, and name
	// the employment fields rather than a pseudo-field.
	f := mustParse(t, `
labels:
  q: { role: quasi-identifier }
  h: { role: attribute }
  who:
    role: direct-identifier
    when: { count: { of: "@quasi-identifier", min: 3 } }
  linked:
    when: { scope: subject, all_of: ["@direct-identifier", h] }
assign:
  employee:
    a: [q]
    b: [q]
    c: [q]
  note:
    text: [h]
`)
	shape := Shape{
		Entities: map[string]TypeShape{
			"employee": {Fields: []string{"a", "b", "c"}},
			"note":     {Fields: []string{"text"}},
		},
		Relations: map[string]RelationShape{"about": {Ends: []Ends{{From: "note", To: "employee"}}}},
	}
	rep := BuildReport(f, shape)
	if len(rep.Subjects) != 1 || len(rep.Subjects[0].Derived) != 1 {
		t.Fatalf("subjects = %+v", rep.Subjects)
	}
	want := []string{"employee.a", "employee.b", "employee.c", "note.text"}
	if got := rep.Subjects[0].Derived[0].Fields; !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
	if !rep.Types[0].IDMayIdentify {
		t.Error("a subject type without opaque ids must be flagged")
	}
}

func TestBuildReport_JSONStable(t *testing.T) {
	f := mustParse(t, workedFile)
	first, err := json.Marshal(BuildReport(f, workedShape()))
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		again, _ := json.Marshal(BuildReport(f, workedShape()))
		if !bytes.Equal(again, first) {
			t.Fatal("report JSON is not stable across runs")
		}
	}
	for _, key := range []string{`"types":`, `"relations":`, `"subjects":`, `"state":"needs-review"`} {
		if !strings.Contains(string(first), key) {
			t.Errorf("JSON lacks %s", key)
		}
	}
}

func TestBuildReport_FieldStates(t *testing.T) {
	f := mustParse(t, "labels: {n: {}}\nassign:\n  person:\n    title: [n]\n    email: none\n    body: needs-review\n")
	shape := Shape{Entities: map[string]TypeShape{"person": {Fields: []string{"title", "email", "phone", "body"}}}}
	got := BuildReport(f, shape).Types[0].Fields
	want := []string{"labeled", "none", "missing", "needs-review"}
	for i, fr := range got {
		if fr.State != want[i] {
			t.Errorf("%s state = %q, want %q", fr.Field, fr.State, want[i])
		}
	}
	if !slices.Equal(got[0].Labels, []string{"n"}) {
		t.Errorf("title labels = %v", got[0].Labels)
	}
}

// A file with an undefined label still reports; the label has no role.
func TestBuildReport_UndefinedLabel(t *testing.T) {
	f, _ := Parse([]byte("assign:\n  person:\n    title: [ghost]\n"))
	rep := BuildReport(f, workedShape())
	if len(rep.Subjects) != 0 {
		t.Errorf("an undefined label made a subject: %+v", rep.Subjects)
	}
}
