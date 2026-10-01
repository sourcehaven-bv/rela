package classification

import (
	"slices"
	"testing"
)

func mustParse(t *testing.T, in string) *File {
	t.Helper()
	f, issues := Parse([]byte(in))
	if len(issues) > 0 {
		t.Fatalf("parse: %v", issues)
	}
	return f
}

func fields(pairs ...string) []Field {
	var out []Field
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Field{Ref: pairs[i], Labels: []string{pairs[i+1]}})
	}
	return out
}

func TestDerive_WorkedExample(t *testing.T) {
	f := mustParse(t, workedExample)
	tests := []struct {
		name   string
		scope  Scope
		fields []Field
		want   []string
	}{
		{"birth date alone is not identifying", ScopeRecord,
			fields("birth_date", "birth-date"), nil},
		{"two quasi-identifiers are not enough", ScopeRecord,
			fields("birth_date", "birth-date", "postcode", "postcode"), nil},
		{"three quasi-identifiers identify", ScopeRecord,
			fields("birth_date", "birth-date", "postcode", "postcode", "gender", "gender"),
			[]string{"identified-person"}},
		{"name with birth date identifies", ScopeRecord,
			fields("title", "name", "birth_date", "birth-date"), []string{"identified-person"}},
		{"name alone is not a combination", ScopeRecord,
			fields("title", "name"), nil},
		{"subject rule ignored at record scope", ScopeRecord,
			fields("title", "name", "diagnosis", "health"), nil},
		{"health linked to a name", ScopeSubject,
			fields("person.title", "name", "sick-leave.diagnosis", "health"), []string{"identified-health"}},
		{"record rule ignored at subject scope", ScopeSubject,
			fields("a", "birth-date", "b", "postcode", "c", "gender"), nil},
		{"no fields", ScopeRecord, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.Derive(tc.scope, tc.fields)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Derive = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDerive_Chains(t *testing.T) {
	// b is derived from a derived label; c needs b's role; declaration order is
	// reversed so a single pass would miss both.
	f := mustParse(t, `
labels:
  x: { role: quasi-identifier }
  c:
    when: { all_of: ["@direct-identifier", x] }
  b:
    role: direct-identifier
    when: { all_of: [a] }
  a:
    when: { all_of: [x] }
`)
	got := f.Derive(ScopeRecord, fields("f", "x"))
	if want := []string{"c", "b", "a"}; !slices.Equal(got, want) {
		t.Errorf("Derive = %v, want %v", got, want)
	}
}

func TestDerive_DoesNotModifyInput(t *testing.T) {
	f := mustParse(t, workedExample)
	in := fields("title", "name", "birth_date", "birth-date")
	in = slices.Clip(in)
	f.Derive(ScopeRecord, in)
	if len(in) != 2 {
		t.Errorf("input changed: %v", in)
	}
}

func TestDerive_MultiLabelField(t *testing.T) {
	f := mustParse(t, workedExample)
	// count counts fields, so one field with three quasi-identifier labels is one.
	got := f.Derive(ScopeRecord, []Field{{Ref: "combined", Labels: []string{"birth-date", "postcode", "gender"}}})
	if len(got) != 0 {
		t.Errorf("Derive = %v; a single field counts once", got)
	}
}

// A derived label stands for fields already present, so it never adds to a
// count: two quasi-identifiers plus a quasi-identifier derived from them are
// still two fields.
func TestDerive_CountIgnoresDerivedFields(t *testing.T) {
	f := mustParse(t, `
labels:
  dob: { role: quasi-identifier }
  zip: { role: quasi-identifier }
  gender: { role: quasi-identifier }
  dob-zip:
    role: quasi-identifier
    when: { all_of: [dob, zip] }
  reidentifiable:
    when: { count: { of: "@quasi-identifier", min: 3 } }
`)
	two := []Field{{Ref: "p.dob", Labels: []string{"dob"}}, {Ref: "p.zip", Labels: []string{"zip"}}}
	if got := f.Derive(ScopeRecord, two); !slices.Equal(got, []string{"dob-zip"}) {
		t.Errorf("two fields: got %v, want [dob-zip]", got)
	}
	three := append(slices.Clone(two), Field{Ref: "p.gender", Labels: []string{"gender"}})
	if got := f.Derive(ScopeRecord, three); !slices.Equal(got, []string{"dob-zip", "reidentifiable"}) {
		t.Errorf("three fields: got %v", got)
	}
}
