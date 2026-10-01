package classification

import (
	"strings"
	"testing"
)

const workedExample = `
labels:
  name:       { role: direct-identifier }
  contact:    { role: direct-identifier }
  birth-date: { role: quasi-identifier }
  postcode:   { role: quasi-identifier }
  gender:     { role: quasi-identifier }
  salary:     { role: attribute, meta: { owner: hr } }
  health:     { role: attribute, description: Physical or mental health. }
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
    body: [health]
subject:
  company: false
subject_link:
  watches: false
subject_hops:
  of: 2
`

func TestParse_WorkedExample(t *testing.T) {
	f, issues := Parse([]byte(workedExample))
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if got := len(f.LabelOrder); got != 9 {
		t.Fatalf("labels = %d, want 9", got)
	}
	if f.Labels["birth-date"].Role != RoleQuasiIdentifier {
		t.Errorf("birth-date role = %q", f.Labels["birth-date"].Role)
	}
	if f.Labels["salary"].Meta["owner"] != "hr" {
		t.Errorf("salary meta = %v", f.Labels["salary"].Meta)
	}
	if r := f.Labels["identified-health"].When; r == nil || r.Scope != ScopeSubject {
		t.Errorf("identified-health rule = %+v", r)
	}
	if r := f.Labels["identified-person"].When; r == nil || r.Scope != ScopeRecord {
		t.Errorf("identified-person rule scope = %+v, want record by default", r)
	}
	if a := f.Assign["person"].Fields["body"]; a.State != None {
		t.Errorf("person.body state = %v, want None", a.State)
	}
	if a := f.Assign["employment"].Fields["postcode"]; a.State != Labeled || a.Labels[0] != "postcode" {
		t.Errorf("employment.postcode = %+v", a)
	}
	if f.Overrides.Subject["company"] || !hasKey(f.Overrides.Subject, "company") {
		t.Errorf("subject override = %v", f.Overrides.Subject)
	}
	if f.Overrides.SubjectHops["of"] != 2 {
		t.Errorf("subject_hops = %v", f.Overrides.SubjectHops)
	}
}

func hasKey[V any](m map[string]V, k string) bool { _, ok := m[k]; return ok }

func TestParse_Empty(t *testing.T) {
	for _, in := range []string{"", "\n", "# only a comment\n"} {
		f, issues := Parse([]byte(in))
		if len(issues) > 0 || f == nil || len(f.Labels) != 0 {
			t.Errorf("Parse(%q) = %v, %v", in, f, issues)
		}
	}
}

func TestParse_Issues(t *testing.T) {
	tests := []struct {
		name string
		in   string
		code string
		path string
		line int
	}{
		{"syntax", "labels: [", CodeSyntax, "", 0},
		{"root not a mapping", "- a\n", CodeInvalidValue, "", 1},
		{"unknown top-level key", "levels: [a]\n", CodeUnknownKey, "levels", 1},
		{"unknown label key", "labels:\n  x: { level: high }\n", CodeUnknownKey, "labels.x.level", 2},
		{"invalid role", "labels:\n  x: { role: identifier }\n", CodeInvalidValue, "labels.x.role", 2},
		{"reserved none", "labels:\n  none: {}\n", CodeReservedName, "labels.none", 2},
		{"reserved needs-review", "labels:\n  needs-review: {}\n", CodeReservedName, "labels.needs-review", 2},
		{"reserved role name", "labels:\n  attribute: {}\n", CodeReservedName, "labels.attribute", 2},
		{"uppercase name", "labels:\n  Health: {}\n", CodeInvalidName, "labels.Health", 2},
		{"space in name", "labels:\n  'a b': {}\n", CodeInvalidName, "labels.a b", 2},
		{"unicode name", "labels:\n  gezondheid-é: {}\n", CodeInvalidName, "labels.gezondheid-é", 2},
		{"duplicate key", "labels:\n  x: {}\n  x: {}\n", CodeDuplicateKey, "labels.x", 3},
		{"non-string key", "labels:\n  1: {}\n", CodeInvalidValue, "labels", 2},
		{"null key", "labels:\n  null: {}\n", CodeInvalidValue, "labels", 2},
		{"anchor", "labels:\n  x: &a {}\n  y: *a\n", CodeYAMLFeature, "labels.x", 2},
		{"merge key", "base: &b {role: attribute}\nlabels:\n  x:\n    <<: *b\n", CodeYAMLFeature, "base", 1},
		{"meta not a mapping", "labels:\n  x: { meta: 3 }\n", CodeInvalidValue, "labels.x.meta", 2},
		{"undefined label in assign", "assign:\n  t:\n    f: [ghost]\n", CodeUndefined, "assign.t.f", 3},
		{"undefined label in rule", "labels:\n  d:\n    when: { all_of: [ghost] }\n", CodeUndefined, "labels.d.when", 2},
		{"derived label assigned", "labels:\n  a: {}\n  d:\n    when: { any_of: [a] }\nassign:\n  t:\n    f: [d]\n",
			CodeInvalidValue, "assign.t.f", 7},
		{"single label as scalar", "labels: {x: {}}\nassign:\n  t:\n    f: x\n", CodeInvalidValue, "assign.t.f", 4},
		{"empty label list", "assign:\n  t:\n    f: []\n", CodeInvalidValue, "assign.t.f", 3},
		{"null field", "assign:\n  t:\n    f:\n", CodeInvalidValue, "assign.t.f", 3},
		{"rule without condition", "labels:\n  d:\n    when: { scope: record }\n", CodeInvalidRule, "labels.d.when", 3},
		{"rule with two conditions", "labels:\n  a: {}\n  d:\n    when: { all_of: [a], any_of: [a] }\n",
			CodeInvalidRule, "labels.d.when", 4},
		{"unknown scope", "labels:\n  a: {}\n  d:\n    when: { scope: output, all_of: [a] }\n",
			CodeInvalidValue, "labels.d.when.scope", 4},
		{"empty all_of", "labels:\n  d:\n    when: { all_of: [] }\n", CodeInvalidRule, "labels.d.when.all_of", 3},
		{"unknown role selector", "labels:\n  d:\n    when: { all_of: ['@name'] }\n",
			CodeInvalidRule, "labels.d.when.all_of[0]", 3},
		{"count without min", "labels:\n  a: {}\n  d:\n    when: { count: { of: a } }\n",
			CodeInvalidRule, "labels.d.when.count", 4},
		{"count min zero", "labels:\n  a: {}\n  d:\n    when: { count: { of: a, min: 0 } }\n",
			CodeInvalidRule, "labels.d.when.count.min", 4},
		{"count min text", "labels:\n  a: {}\n  d:\n    when: { count: { of: a, min: three } }\n",
			CodeInvalidRule, "labels.d.when.count.min", 4},
		{"subject override not bool", "subject:\n  person: yes please\n", CodeInvalidValue, "subject.person", 2},
		{"hops too large", "subject_hops:\n  of: 4\n", CodeLimit, "subject_hops.of", 2},
		{"hops zero", "subject_hops:\n  of: 0\n", CodeLimit, "subject_hops.of", 2},
		{"profiles are slice 2", "profiles: [org]\n", CodeUnknownKey, "profiles", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := Parse([]byte(tc.in))
			for _, is := range issues {
				if is.Code == tc.code && is.Path == tc.path && is.Line == tc.line {
					return
				}
			}
			t.Fatalf("want issue %s at %q line %d, got %v", tc.code, tc.path, tc.line, issues)
		})
	}
}

func TestParse_Limits(t *testing.T) {
	t.Run("file size", func(t *testing.T) {
		_, issues := Parse(make([]byte, MaxFileSize+1))
		if len(issues) != 1 || issues[0].Code != CodeLimit {
			t.Fatalf("issues = %v", issues)
		}
	})
	t.Run("label count", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("labels:\n")
		for i := range MaxLabels + 1 {
			b.WriteString("  l" + itoa3(i) + ": {}\n")
		}
		b.WriteString("assign:\n  person:\n    email: [l000]\n")
		_, issues := Parse([]byte(b.String()))
		// The labels within the cap are still read, so a reference to one
		// does not cascade into an undefined-label issue.
		if len(issues) != 1 || issues[0].Code != CodeLimit {
			t.Fatalf("issues = %v", issues)
		}
	})
	t.Run("rule depth", func(t *testing.T) {
		rule := "a"
		for range MaxRuleDepth {
			rule = "{ all_of: [" + rule + "] }"
		}
		_, issues := Parse([]byte("labels:\n  a: {}\n  d:\n    when: " + rule + "\n"))
		if !hasCode(issues, CodeLimit) {
			t.Fatalf("issues = %v", issues)
		}
	})
	t.Run("rule depth at the limit", func(t *testing.T) {
		rule := "a"
		for range MaxRuleDepth - 1 {
			rule = "{ all_of: [" + rule + "] }"
		}
		_, issues := Parse([]byte("labels:\n  a: {}\n  d:\n    when: " + rule + "\n"))
		if len(issues) > 0 {
			t.Fatalf("issues = %v", issues)
		}
	})
}

func itoa3(i int) string {
	return string([]byte{byte('0' + i/100), byte('0' + i/10%10), byte('0' + i%10)})
}

func hasCode(issues []Issue, code string) bool {
	for _, is := range issues {
		if is.Code == code {
			return true
		}
	}
	return false
}

func TestIssue_String(t *testing.T) {
	got := Issue{Code: CodeNeedsReview, Path: "assign.t.f", Line: 7, Message: "m"}.String()
	if got != "classification.yaml:7: assign.t.f: m" {
		t.Errorf("String() = %q", got)
	}
	got = Issue{Path: "assign.t", Message: "m"}.String()
	if got != "classification.yaml: assign.t: m" {
		t.Errorf("String() = %q", got)
	}
}
