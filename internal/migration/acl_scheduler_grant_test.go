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
			src: "roles:\n  scheduler-system:\n    read: [\"*\"]\n" +
				"assignments:\n  system:scheduler: scheduler-system\n",
			want: false,
		},
		{
			name: "scheduler assigned to an operator's own role",
			src: "roles:\n  my-custom-role:\n    read: [ticket]\n" +
				"assignments:\n  system:scheduler: my-custom-role\n",
			want: false,
		},
		{
			name: "empty document",
			src:  "",
			want: false,
		},
		// The shapes that previously produced a silent false success.
		{
			name: "roles present but null",
			src:  "roles:\nassignments:\n  alice: viewer\n",
			want: true,
		},
		{
			name: "assignments present but null",
			src:  "roles:\n  viewer:\n    read: [ticket]\nassignments:\n",
			want: true,
		},
		{
			name: "assignment names a role that is not defined",
			src:  "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  system:scheduler: ghost\n",
			want: true,
		},
		{
			name: "assigned role grants no read",
			src: "roles:\n  writer:\n    create: [ticket]\n" +
				"assignments:\n  system:scheduler: writer\n",
			want: true,
		},
		{
			name: "granted via asserted_role_assignments (scalar)",
			src: "roles:\n  reporting:\n    read: [ticket]\n" +
				"asserted_role_assignments:\n  system:scheduler: reporting\n",
			want: false,
		},
		{
			name: "granted via asserted_role_assignments (list)",
			src: "roles:\n  reporting:\n    read: [ticket]\n" +
				"asserted_role_assignments:\n  system:scheduler: [other, reporting]\n",
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
// Runs over the awkward shapes too: idempotency is precisely the property
// that broke on a null `assignments:` (Detect stayed true forever, so every
// `rela migrate` re-applied and re-wrote the file).
func TestACLSchedulerGrant_ApplyIsIdempotent(t *testing.T) {
	sources := map[string]string{
		"well formed":      "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n",
		"roles null":       "roles:\nassignments:\n  alice: viewer\n",
		"assignments null": "roles:\n  viewer:\n    read: [ticket]\nassignments:\n",
		"both null":        "roles:\nassignments:\n",
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			m := &migration.ACLSchedulerGrantMigration{}
			doc := parseDoc(t, src)
			applyIdempotently(t, m, doc)
		})
	}
}

func applyIdempotently(t *testing.T, m *migration.ACLSchedulerGrantMigration, doc *yaml.Node) {
	t.Helper()
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if m.Detect(doc) {
		out, _ := yaml.Marshal(doc)
		t.Errorf("Detect still true after Apply — migration never converges:\n%s", out)
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
		// `key:` with nothing under it parses as a NULL SCALAR, not an
		// empty mapping. Both of these previously yielded a file that
		// parsed and validated while granting nothing.
		"roles null":          "roles:\nassignments:\n  alice: viewer\n",
		"assignments null":    "roles:\n  viewer:\n    read: [ticket]\nassignments:\n",
		"both null":           "roles:\nassignments:\n",
		"dangling assignment": "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  system:scheduler: ghost\n",
		"assigned role grants no read": "roles:\n  writer:\n    create: [ticket]\n" +
			"assignments:\n  system:scheduler: writer\n",
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
			// A role WE mint must be read-only — the migration must never
			// hand the scheduler write verbs it did not already have. A
			// role the operator wrote is theirs; we only ensure it reads.
			writes := len(def.Update) + len(def.Delete) + len(def.Create)
			if role == "scheduler-system" && writes != 0 {
				t.Errorf("migration-created role grants write verbs (create=%v update=%v delete=%v)",
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
	// Direct and unconditional: this holds regardless of whether Apply
	// works on any particular input, so a structural bug elsewhere cannot
	// masquerade as "the literal is fine".
	if migration.SchedulerPrincipal != principal.UserScheduler {
		t.Fatalf("migration.SchedulerPrincipal = %q but principal.UserScheduler = %q; "+
			"the migration would grant a principal nothing runs as",
			migration.SchedulerPrincipal, principal.UserScheduler)
	}
}

// TestACLSchedulerGrant_RespectsAssertedRoleGrant: an operator who scoped
// the scheduler through asserted_role_assignments has already thought about
// this. Do not pile a read:["*"] role on top of their narrower grant.
func TestACLSchedulerGrant_RespectsAssertedRoleGrant(t *testing.T) {
	src := "roles:\n  reporting:\n    read: [ticket]\n" +
		"asserted_role_assignments:\n  system:scheduler: reporting\n"
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, src)

	if m.Detect(doc) {
		t.Fatal("Detect true despite an existing asserted read grant — would widen the operator's scope")
	}
}

// TestACLSchedulerGrant_RepairsDeadRole: a role that exists but grants no
// read must be given one, not silently bound to. Binding to an empty role
// just relocates the bug.
func TestACLSchedulerGrant_RepairsDeadRole(t *testing.T) {
	src := "roles:\n  scheduler-system: {}\nassignments:\n  alice: viewer\n"
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
	if got := policy.Roles["scheduler-system"].Read; len(got) == 0 {
		t.Errorf("dead role left without read:\n%s", out)
	}
}

// TestACLSchedulerGrant_ApplyFailsLoudlyWhenItCannotGrant exercises the
// postcondition. Every bug this migration has had produced a file that
// parsed and validated while granting nothing, so Apply must verify its
// own result rather than trust that it wrote something.
//
// The input is a malformed policy the repair cannot fix: the scheduler's
// assignment value is a nested mapping, not a role name. Returning an
// error stops the runner before it writes, so the operator's file survives
// (see TestMigrate_MalformedACLDoesNotClobber for the on-disk half).
func TestACLSchedulerGrant_ApplyFailsLoudlyWhenItCannotGrant(t *testing.T) {
	src := "roles:\n  viewer:\n    read: [ticket]\n" +
		"assignments:\n  system:scheduler:\n    nested: yes\n"
	m := &migration.ACLSchedulerGrantMigration{}
	doc := parseDoc(t, src)

	err := m.Apply(doc)
	if err == nil {
		out, _ := yaml.Marshal(doc)
		t.Fatalf("Apply reported success but granted nothing:\n%s", out)
	}
	if !strings.Contains(err.Error(), migration.SchedulerPrincipal) {
		t.Errorf("error should name the principal, got: %v", err)
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
