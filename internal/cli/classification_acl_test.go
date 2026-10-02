package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclaudit"
	"github.com/Sourcehaven-BV/rela/internal/classification"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

const classificationTestPolicy = `
roles:
  everyone:
    read: [topic]
  staff:
    read: [person, topic]
  support:
    read: [person, topic]
    visible:
      person:
        - field: email
          when: "entity.name == 'nobody'"
client_baselines:
  apps:
    applies_to: [app]
    redact:
      topic: [title]
scope_grants:
  rela.topics.read:
    visible:
      topic: [title]
`

// support's email grant is conditional; the audit counts it as granted.

func classificationTestMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	mm, err := metamodel.Parse([]byte(classificationTestSchema))
	if err != nil {
		t.Fatal(err)
	}
	return mm
}

func classificationTestPolicyParsed(t *testing.T) *acl.Policy {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acl.yaml")
	if err := os.WriteFile(path, []byte(classificationTestPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClassificationViews(t *testing.T) {
	var got []string
	for _, v := range classificationViews(classificationTestPolicyParsed(t)) {
		got = append(got, v.String())
	}
	want := []string{
		"role everyone",
		"role everyone as app (no scopes)",
		"role everyone as app (scopes: rela.topics.read)",
		"role staff",
		"role staff as app (no scopes)",
		"role staff as app (scopes: rela.topics.read)",
		"role support",
		"role support as app (no scopes)",
		"role support as app (scopes: rela.topics.read)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("views:\n got %q\nwant %q", got, want)
	}
}

func TestClassificationViewFor(t *testing.T) {
	meta := classificationTestMeta(t)
	policy := classificationTestPolicyParsed(t)
	shape := classificationShape(meta)
	cases := []struct {
		view      classificationView
		entities  map[string][]string
		relations map[string][]string
	}{
		{
			view:     classificationView{role: acl.EveryoneRole},
			entities: map[string][]string{"topic": {"title", "body"}},
		},
		{
			view:      classificationView{role: "staff"},
			entities:  map[string][]string{"person": {"name", "email", "body"}, "topic": {"title", "body"}},
			relations: map[string][]string{"follows": {"since", "body"}},
		},
		{
			view:      classificationView{role: "support"},
			entities:  map[string][]string{"person": {"email", "body"}, "topic": {"title", "body"}},
			relations: map[string][]string{"follows": {"since", "body"}},
		},
		{
			view:      classificationView{role: "staff", principalType: "app"},
			entities:  map[string][]string{"person": {"name", "email", "body"}, "topic": {"body"}},
			relations: map[string][]string{"follows": {"since", "body"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.view.String(), func(t *testing.T) {
			got, err := classificationViewFor(context.Background(), policy, meta, shape, tc.view)
			if err != nil {
				t.Fatal(err)
			}
			if !mapsEqual(got.Entities, tc.entities) {
				t.Errorf("entities = %v, want %v", got.Entities, tc.entities)
			}
			if !mapsEqual(got.Relations, tc.relations) {
				t.Errorf("relations = %v, want %v", got.Relations, tc.relations)
			}
		})
	}
}

func mapsEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !slices.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

func TestClassificationFindings(t *testing.T) {
	meta := classificationTestMeta(t)
	f, issues := classification.Parse([]byte(classificationTestFile))
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	findings, err := classificationFindings(context.Background(), classificationTestPolicyParsed(t), meta, f)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, fd := range findings {
		if fd.Severity != aclaudit.Low {
			t.Errorf("%s: severity %s, want low", fd.Rule, fd.Severity)
		}
		got = append(got, fd.Rule+" "+fd.Subject+": "+fd.Detail)
	}
	// everyone's C1 on topic is listed once, not again per role; each client
	// view either lists its own findings or says it matches or reads none.
	want := []string{
		"C1-role-reads-label topic: role everyone reads topic-interest data in topic.title",
		"C0-reads-none role everyone as app (no scopes): role everyone as app (no scopes) reads no labeled data",
		"C0-same-as-role role everyone as app (scopes: rela.topics.read): " +
			"role everyone as app (scopes: rela.topics.read) reads the same labeled data as the role alone",
		"C1-role-reads-label person: role staff reads name data in person.name",
		"C3-derived-subject subject person: role staff can combine person.name, topic.title " +
			"in data about one person into interested",
		"C1-role-reads-label person: role staff as app (no scopes) reads name data in person.name",
		"C0-same-as-role role staff as app (scopes: rela.topics.read): " +
			"role staff as app (scopes: rela.topics.read) reads the same labeled data as the role alone",
		"C0-reads-none role support as app (no scopes): role support as app (no scopes) reads no labeled data",
		"C0-same-as-role role support as app (scopes: rela.topics.read): " +
			"role support as app (scopes: rela.topics.read) reads the same labeled data as the role alone",
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings:\n got %q\nwant %q", got, want)
	}
	for _, fd := range findings {
		if fd.Rule == classification.RuleDerivedSubject &&
			fd.Fix != "hiding any one of person.name, topic.title breaks the combination" {

			t.Errorf("fix = %q", fd.Fix)
		}
	}
}

func TestACLAudit_Classification(t *testing.T) {
	svc := aclTestServices(t, classificationTestMeta(t), classificationTestPolicy)
	path := filepath.Join(svc.Paths.Root, classification.FileName)
	if err := os.WriteFile(path, []byte(classificationTestFile), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(cmd *ACLAuditCmd) string {
		t.Helper()
		buf := withOutput(t, output.FormatTable)
		if err := cmd.Run(svc); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	if got := run(&ACLAuditCmd{}); !strings.Contains(got, "role staff can combine person.name, topic.title") {
		t.Errorf("audit lacks the subject finding:\n%s", got)
	}
	buf := withOutput(t, output.FormatJSON)
	if err := (&ACLAuditCmd{}).Run(svc); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Details []auditFindingJSON `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(res.Details, func(d auditFindingJSON) bool {
		return d.Label == "interested" && slices.Equal(d.Fields, []string{"person.name", "topic.title"})
	}) {

		t.Errorf("JSON lacks structured label and fields: %+v", res.Details)
	}

	if got := run(&ACLAuditCmd{NoClassification: true}); strings.Contains(got, "C1-role-reads-label") {
		t.Errorf("--no-classification still lists classification findings:\n%s", got)
	}

	var warnings bytes.Buffer
	prev := classificationWarnings
	classificationWarnings = &warnings
	t.Cleanup(func() { classificationWarnings = prev })
	for _, bad := range []struct{ name, content string }{
		{"broken", "labels: [\n"},
		{"oversized", "# " + strings.Repeat("x", classification.MaxFileSize) + "\n"},
	} {
		warnings.Reset()
		if err := os.WriteFile(path, []byte(bad.content), 0o600); err != nil {
			t.Fatal(err)
		}
		got := run(&ACLAuditCmd{})
		if !strings.Contains(warnings.String(), "skipping its findings") {
			t.Errorf("%s file: no warning on stderr: %q", bad.name, warnings.String())
		}
		if strings.Contains(got, "skipping") || strings.Contains(got, "C1-role-reads-label") {
			t.Errorf("%s file: warning or findings in the report:\n%s", bad.name, got)
		}
	}
}

func TestExposureFinding(t *testing.T) {
	v := classificationView{role: "staff"}
	cases := []struct {
		name       string
		e          classification.Exposure
		detail     string
		fixContain string
	}{
		{
			name:       "record with breakers",
			e:          classification.Exposure{Rule: classification.RuleDerivedRecord, Subject: "person", Label: "x", Fields: []string{"person.a", "person.b"}, Breakers: []string{"person.a"}},
			detail:     "role staff can combine person.a, person.b in one record into x",
			fixContain: "hiding any one of person.a",
		},
		{
			name:       "two removals",
			e:          classification.Exposure{Rule: classification.RuleDerivedRecord, Subject: "person", Label: "x", Fields: []string{"person.a", "person.b"}, MinRemovals: 2},
			detail:     "role staff can combine person.a, person.b in one record into x",
			fixContain: "hiding 2 of them together does",
		},
		{
			name:       "beyond the search",
			e:          classification.Exposure{Rule: classification.RuleDerivedRecord, Subject: "person", Label: "x", Fields: []string{"person.a", "person.b"}},
			detail:     "role staff can combine person.a, person.b in one record into x",
			fixContain: "more than 3 hidden fields",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exposureFinding(v, tc.e)
			if got.Detail != tc.detail || !strings.Contains(got.Fix, tc.fixContain) {
				t.Errorf("got %q / %q", got.Detail, got.Fix)
			}
		})
	}
}

// Classification findings are low severity but never trip the gate, even at
// --fail-on=any: they describe access an operator may intend.
func TestACLAudit_ClassificationDoesNotGate(t *testing.T) {
	const policy = `
user_entity_type: person
roles:
  staff:
    read: [person, topic]
`
	svc := aclTestServices(t, classificationTestMeta(t), policy)
	path := filepath.Join(svc.Paths.Root, classification.FileName)
	if err := os.WriteFile(path, []byte(classificationTestFile), 0o600); err != nil {
		t.Fatal(err)
	}
	buf := withOutput(t, output.FormatTable)
	if err := (&ACLAuditCmd{FailOn: "any"}).Run(svc); err != nil {
		t.Fatalf("classification findings tripped the gate: %v\n%s", err, buf)
	}
	if !strings.Contains(buf.String(), "C1-role-reads-label") {
		t.Errorf("no classification findings listed:\n%s", buf)
	}
}

func findingsFor(t *testing.T, policyYAML, file string) []aclaudit.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acl.yaml")
	if err := os.WriteFile(path, []byte(policyYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	f, issues := classification.Parse([]byte(file))
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	out, err := classificationFindings(context.Background(), policy, classificationTestMeta(t), f)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func hasFinding(fs []aclaudit.Finding, rule, detailPrefix string) bool {
	return slices.ContainsFunc(fs, func(f aclaudit.Finding) bool {
		return f.Rule == rule && strings.HasPrefix(f.Detail, detailPrefix)
	})
}

// A role's view includes what `everyone` grants, so a subject combination
// that needs both is reported for the role.
func TestClassificationFindings_EveryoneJoin(t *testing.T) {
	fs := findingsFor(t, `
roles:
  everyone:
    read: [person]
  analyst:
    read: [topic]
`, classificationTestFile)
	if !hasFinding(fs, classification.RuleDerivedSubject, "role analyst can combine person.name, topic.title") {
		t.Errorf("analyst's join with everyone's grant is not reported: %+v", fs)
	}
	if hasFinding(fs, classification.RuleDerivedSubject, "role everyone") {
		t.Errorf("everyone alone cannot read topic: %+v", fs)
	}
}

// A relation's `visible:` grant decides which of its fields a view reads.
func TestClassificationFindings_RelationVisible(t *testing.T) {
	const file = `labels:
  name: { role: direct-identifier }
  history: {}
assign:
  person: { name: [name], email: none, body: none }
  topic: { title: none, body: none }
assign_relations:
  follows: { since: [history], body: [history] }
`
	const policy = `
roles:
  open:
    read: [person, topic]
  closed:
    read: [person, topic]
    relations:
      person:
        - relation: follows
          visible:
            - field: note
`
	fs := findingsFor(t, policy, file)
	if !hasFinding(fs, classification.RuleReadsLabel, "role open reads history data in ~follows.since, ~follows.body") {
		t.Errorf("open reads the relation field: %+v", fs)
	}
	// The body is served with the edge whatever visible: says.
	if !hasFinding(fs, classification.RuleReadsLabel, "role closed reads history data in ~follows.body") {
		t.Errorf("closed's relation visible: hides since but not the body: %+v", fs)
	}
}

// A policy that resolves principals by property still audits: the synthetic
// principal needs no lookup.
func TestClassificationFindings_PrincipalProperty(t *testing.T) {
	fs := findingsFor(t, `
user_entity_type: person
principal_property: email
roles:
  staff:
    read: [person]
`, classificationTestFile)
	if !hasFinding(fs, classification.RuleReadsLabel, "role staff reads name data in person.name") {
		t.Errorf("findings: %+v", fs)
	}
}
