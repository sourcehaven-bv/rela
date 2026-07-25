package migration_test

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-76JP2A / RR-1USMEZ: scheduled tasks resolve reads against a fixed
// system identity. A project with an acl.yaml that never assigns that
// identity a role has tasks reading nothing; this migration repairs it.

func parseDoc(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &doc
}

func TestACLSchedulerGrant_Detect(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "no assignments block",
			src:  "roles:\n  viewer:\n    read: [ticket]\n",
			want: true,
		},
		{
			name: "assignments without the scheduler",
			src:  "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n",
			want: true,
		},
		{
			name: "scheduler already assigned",
			src:  "assignments:\n  system:scheduler: scheduler-system\n",
			want: false,
		},
		{
			name: "scheduler assigned to an operator's own role",
			src:  "assignments:\n  system:scheduler: my-custom-role\n",
			want: false,
		},
		{
			name: "empty document",
			src:  "",
			want: false,
		},
	}

	m := &migration.ACLSchedulerGrantMigration{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Detect(parseDoc(t, tc.src)); got != tc.want {
				t.Errorf("Detect() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestACLSchedulerGrant_ApplyIsIdempotent pins the package convention:
// after Apply, Detect must be false and a second Apply must not duplicate.
func TestACLSchedulerGrant_ApplyIsIdempotent(t *testing.T) {
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n")

	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if m.Detect(doc) {
		t.Error("Detect still true after Apply")
	}

	first, marshalErr := yaml.Marshal(doc)
	if marshalErr != nil {
		t.Fatalf("marshal: %v", marshalErr)
	}
	if applyErr := m.Apply(doc); applyErr != nil {
		t.Fatalf("second Apply: %v", applyErr)
	}
	second, marshalErr := yaml.Marshal(doc)
	if marshalErr != nil {
		t.Fatalf("marshal: %v", marshalErr)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("second Apply changed the document:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestACLSchedulerGrant_PreservesComments is the whole point of the
// yaml.Node approach — an operator's policy file keeps its comments and
// its existing entries.
func TestACLSchedulerGrant_PreservesComments(t *testing.T) {
	src := `# Our access policy - keep in sync with the handbook.
roles:
  # Analysts get read-only access.
  viewer:
    read: [ticket]
assignments:
  alice: viewer
`
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, src)
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		"# Our access policy - keep in sync with the handbook.",
		"# Analysts get read-only access.",
		"alice: viewer",
		"scheduler-system",
		"system:scheduler",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

// TestACLSchedulerGrant_ProducesLoadablePolicy is the guard against
// shipping a file that blocks boot: appbuild hard-fails on a malformed or
// invalid acl.yaml, so a migration that generated one would be worse than
// the bug it fixes.
func TestACLSchedulerGrant_ProducesLoadablePolicy(t *testing.T) {
	sources := map[string]string{
		"with existing roles": "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n",
		"assignments only":    "assignments:\n  alice: viewer\n",
		"roles only":          "roles:\n  viewer:\n    read: [ticket]\n",
	}

	m := &migration.ACLSchedulerGrantMigration{}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			doc := parseDoc(t, src)
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			out, err := yaml.Marshal(doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var policy acl.Policy
			if err := yaml.Unmarshal(out, &policy); err != nil {
				t.Fatalf("migrated policy does not parse: %v\n%s", err, out)
			}
			if err := policy.Validate(); err != nil {
				t.Fatalf("migrated policy fails Validate (would block boot): %v\n%s", err, out)
			}

			// The grant must actually bind the principal the scheduler uses.
			role, ok := policy.Assignments[principal.UserScheduler]
			if !ok {
				t.Fatalf("no assignment for %q:\n%s", principal.UserScheduler, out)
			}
			def, ok := policy.Roles[role]
			if !ok {
				t.Fatalf("assignment names role %q which is not defined:\n%s", role, out)
			}
			if len(def.Read) == 0 {
				t.Errorf("role %q grants no read:\n%s", role, out)
			}
			// Read-only: a privilege grant must not smuggle in write verbs.
			if len(def.Update) != 0 || len(def.Delete) != 0 || len(def.Create) != 0 {
				t.Errorf("scheduler role grants write verbs (create=%v update=%v delete=%v)",
					def.Create, def.Update, def.Delete)
			}
		})
	}
}

// TestACLSchedulerGrant_RespectsOperatorRole: if the operator already
// defines a role by our name, bind to theirs rather than overwriting it.
func TestACLSchedulerGrant_RespectsOperatorRole(t *testing.T) {
	src := "roles:\n  scheduler-system:\n    read: [ticket]\nassignments:\n  alice: viewer\n"
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, src)
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var policy acl.Policy
	if err := yaml.Unmarshal(out, &policy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Their narrower definition survives; we did not widen it to "*".
	got := policy.Roles["scheduler-system"].Read
	if len(got) != 1 || got[0] != "ticket" {
		t.Errorf("operator's role was overwritten: read = %v, want [ticket]", got)
	}
	if policy.Assignments[principal.UserScheduler] != "scheduler-system" {
		t.Errorf("assignment = %q, want scheduler-system", policy.Assignments[principal.UserScheduler])
	}
}

// TestACLSchedulerGrant_PrincipalMatchesRuntime pins the literal in this
// package against the constant the scheduler actually stamps. internal/
// migration deliberately imports nothing but yaml, so the value is
// duplicated — this test is what keeps the copy honest. If it fails, the
// migration is writing a grant for a principal nothing runs as.
func TestACLSchedulerGrant_PrincipalMatchesRuntime(t *testing.T) {
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, "assignments:\n  alice: viewer\n")
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var policy acl.Policy
	if err := yaml.Unmarshal(out, &policy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := policy.Assignments[principal.UserScheduler]; !ok {
		t.Fatalf("migration grants no role to principal.UserScheduler (%q); "+
			"the literal in acl_scheduler_grant.go has drifted:\n%s",
			principal.UserScheduler, out)
	}
}

// TestACLSchedulerGrant_Registered pins the wiring: an unregistered
// migration silently never runs.
func TestACLSchedulerGrant_Registered(t *testing.T) {
	for _, m := range migration.ForFileType(migration.FileTypeACL) {
		if m.Name() == "acl-scheduler-grant" {
			return
		}
	}
	t.Fatal("acl-scheduler-grant is not registered for FileTypeACL")
}
